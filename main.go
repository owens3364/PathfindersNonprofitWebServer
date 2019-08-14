package main

import (
	"fmt"
	"log"
	"net/http"
	"net/smtp"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-contrib/gzip"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/charge"
	"github.com/stripe/stripe-go/paymentintent"
)

// For creating Stripe payments
type PaymentData struct {
	Amount      int64  `json:"amount" binding:"required"`
	Description string `json:"description" binding:"required"`
	Name        string `json:"name" binding:"required"`
	Addr1       string `json:"addr1" binding:"required"`
	Addr2       string `json:"addr2" binding:"required"`
	City        string `json:"city" binding:"required"`
	State       string `json:"state" binding:"required"`
	Zip         string `json:"zip" binding:"required"`
	Email       string `json:"email" binding:"required"`
	Phone       string `json:"phone" binding:"required"`
}

type Token struct {
	Amount      int64  `json:"amount" binding:"required"`
	Description string `json:"description" binding:"required"`
	Name        string `json:"name" binding:"required"`
	Addr1       string `json:"addr1" binding:"required"`
	Addr2       string `json:"addr2" binding:"required"`
	City        string `json:"city" binding:"required"`
	State       string `json:"state" binding:"required"`
	Zip         string `json:"zip" binding:"required"`
	Email       string `json:"email" binding:"required"`
	Phone       string `json:"phone" binding:"required"`
	StripeToken string `json:"token" binding:"required"`
}

func main() {
	router := gin.Default()
	fmt.Println("Router instance created")

	// Ping functionality

	pingFunctionality, pingErr := strconv.ParseBool(os.Getenv("PING_FUNCTIONALITY"))
	if pingErr == nil {
		if pingFunctionality {
			router.GET("/ping", func(c *gin.Context) { c.String(200, "pong "+fmt.Sprint(time.Now().Unix())) })
			fmt.Println("Ping functionality established at /ping")
		} else {
			fmt.Println("Ping functionality at /ping disabled")
		}
	} else {
		fmt.Println(pingErr)
		fmt.Println("The environment variable 'PING_FUNCTIONALITY' did not have a valid 'true' or 'false' value. Ensure the 'PING_FUNCTIONALITY' key is present and has a value of either 'true' or 'false'. All functionality at /ping is currently disabled.")
	}

	// Ad hoc custom security middleware
	// Redirects http to https
	// Also ensures the site is not used in a clickjacking attack
	// I may add more security features to this
	// Cloudflare is our DNS, and requests are first routed through the Cloudeflare CDN before they come to Heroku
	// This provides DDOS protection as well as HSTS headers, which is why they are not included here

	// NOTE: CSRF protection must be added once the site gets users and stuff

	router.Use(func() gin.HandlerFunc {
		return func(c *gin.Context) {
			c.Header("X-Frame-Options", "deny")
			c.Header("X-XSS-Protection", "1; mode=block")
			c.Header("Content-Security-Policy", "default-src 'none'; script-src 'self' https://www.google-analytics.com https://js.stripe.com https://ajax.cloudflare.com 'sha256-5As4+3YpY62+l38PsxCEkjB1R4YtyktBtRScTJ3fyLU='; style-src 'self' https://maxcdn.bootstrapcdn.com 'sha256-oWyvTH6ZfCvIDieRREt+hfVBWcf5gzk2WAW4xELF74Q='; img-src 'self'; connect-src 'self' https://js.stripe.com https://www.google-analytics.com https://www.youtube.com https://m.stripe.network https://q.stripe.com; media-src https://www.youtube.com; child-src https://www.youtube.com https://js.stripe.com https://m.stripe.network; form-action 'self'; frame-ancestors 'none';")
			if os.Getenv("GIN_MODE") == "release" {
				if c.Request.Header.Get("X-Forwarded-Proto") != "https" {
					c.Redirect(http.StatusMovedPermanently, "https://www.pathfindersrobotics.org"+c.Request.URL.Path)
				}
			}
		}
	}())
	fmt.Println("HTTP --> HTTPS redirection enabled when environment variable 'GIN_MODE' is 'release'.")

	// Static serve site under gzip compression
	compressionLevel := gzip.DefaultCompression
	fmt.Println("Using Gzip Compression based on the 'GZIP_COMPRESSION_LVL' environment variable.")
	fmt.Println("Options for compression include 'BestCompression', 'BestSpeed', 'NoCompression', and 'DefaultCompression'. Default Gzip compression will be used if the 'GZIP_COMPRESSION_LVL' environment variable is unset.")
	if os.Getenv("GZIP_COMPRESSION_LVL") == "BestCompression" {
		compressionLevel = gzip.BestCompression
	} else if os.Getenv("GZIP_COMPRESSION_LVL") == "BestSpeed" {
		compressionLevel = gzip.BestSpeed
	} else if os.Getenv("GZIP_COMPRESSION_LVL") == "NoCompression" {
		compressionLevel = gzip.NoCompression
	}
	router.Use(gzip.Gzip(compressionLevel))

	// Have a cache policy of one year for index.html and static assets
	router.Use(func() gin.HandlerFunc {
		return func(c *gin.Context) {
			path := c.Request.URL.Path
			if string([]rune(path)[0:1]) != "/" {
				path = "/" + path
			}
			if path == "/" || string([]rune(path)[0:7]) == "/static" {
				c.Header("Cache-Control", "max-age=31536000")
			} else {
				c.Header("Cache-Control", "no-cache")
			}
		}
	}())

	// Static serve site

	siteServing, siteErr := strconv.ParseBool(os.Getenv("SERVING_SITE"))
	if siteErr == nil {
		if siteServing {
			router.Use(static.Serve("/", static.LocalFile("./static", true)))
			fmt.Println("Site is being served per the 'SERVING_SITE' environment variable")
		} else {
			fmt.Println("Site is not being served, per the 'SERVING_SITE' environment variable.")
		}
	} else {
		fmt.Println("The environment variable 'SERVING_SITE' did not have a valid 'true' or 'false' value. Ensure the 'SERVING_SITE' key is present and has a value of 'true' or 'false'. All site serving functionality is currently disabled.")
	}

	// Email Payment notifications

	notifications, notifErr := strconv.ParseBool(os.Getenv("EMAIL_PAYMENT_NOTIFICATIONS"))
	if notifErr != nil {
		fmt.Println("The environment variable 'EMAIL_PAYMENT_NOTIFICATIONS' did not have a valid 'true' or 'false' value. Ensure the 'EMAIL_PAYMENT_NOTIFICATIONS' key is present and has a value of either 'true' or 'false'. All Payment Email deliverance functionality is currently disabled.")
	} else {
		if notifications {
			router.POST("/paymentEmail", func(c *gin.Context) {
				var data PaymentData
				c.BindJSON(&data)
				sendPaymentEmail(&data)
				c.String(200, "OK")
			})
			fmt.Println("The payment email functionalities at /paymentEmail are currently enabled, per the 'EMAIL_PAYMENT_NOTIFICATIONS' environment variable.")
		} else {
			fmt.Println("The payment email functionalities at /paymentEmail are currently disabled, per the 'EMAIL_PAYMENT_NOTIFICATIONS' environment variable.")
		}
	}

	// Handle Stripe payments

	stripeLive, err := strconv.ParseBool(os.Getenv("STRIPE_LIVE"))
	if err == nil {
		fmt.Println("The environment variable 'STRIPE_LIVE' was found with a valid 'true' or 'false' attribute.")
		if stripeLive {
			stripe.Key = os.Getenv("STRIPE_LIVE_KEY")
			fmt.Println("Stripe functionality is enabled in LIVE mode.")
		} else {
			stripe.Key = os.Getenv("STRIPE_DEBUG_KEY")
			fmt.Println("Stripe functionality is enabled in DEBUG mode.")
		}

		router.POST("/getSecret", func(c *gin.Context) {
			card := "card"
			cardPointer := &card
			var paymentIntentData PaymentData
			c.BindJSON(&paymentIntentData)
			intent, _ := paymentintent.New(&stripe.PaymentIntentParams{
				Amount:      stripe.Int64(paymentIntentData.Amount),
				Currency:    stripe.String(string(stripe.CurrencyUSD)),
				Description: stripe.String(paymentIntentData.Description),
				PaymentMethodTypes: []*string{
					cardPointer,
				},
				ReceiptEmail: stripe.String(paymentIntentData.Email),
				Shipping: &stripe.ShippingDetailsParams{
					Address: &stripe.AddressParams{
						City:       stripe.String(paymentIntentData.City),
						Country:    stripe.String("US"),
						Line1:      stripe.String(paymentIntentData.Addr1),
						Line2:      stripe.String(paymentIntentData.Addr2),
						PostalCode: stripe.String(paymentIntentData.Zip),
						State:      stripe.String(paymentIntentData.State),
					},
					Name:  stripe.String(paymentIntentData.Name),
					Phone: stripe.String(paymentIntentData.Phone),
				},
			})
			c.JSON(200, gin.H{
				"secret": intent.ClientSecret,
			})
		})

		router.POST("/paymentRequest", func(c *gin.Context) {
			var token Token
			c.BindJSON(&token)
			params := &stripe.ChargeParams{
				Amount:       stripe.Int64(token.Amount),
				Currency:     stripe.String(string(stripe.CurrencyUSD)),
				Description:  stripe.String(token.Description),
				ReceiptEmail: stripe.String(token.Email),
				Shipping: &stripe.ShippingDetailsParams{
					Address: &stripe.AddressParams{
						City:       stripe.String(token.City),
						Country:    stripe.String("US"),
						Line1:      stripe.String(token.Addr1),
						Line2:      stripe.String(token.Addr2),
						PostalCode: stripe.String(token.Zip),
						State:      stripe.String(token.State),
					},
					Name:  &token.Name,
					Phone: &token.Phone,
				},
			}
			params.SetSource(token.StripeToken)
			ch, _ := charge.New(params)
			if ch.Paid {
				c.JSON(200, gin.H{
					"success": true,
				})
				go sendPaymentEmail(tokenToPaymentData(&token))
			} else {
				c.JSON(200, gin.H{
					"success": false,
				})
			}
		})

	} else {
		fmt.Println(err)
		fmt.Println("The environment variable 'STRIPE_LIVE' did not have a valid 'true' or 'false' value. Ensure the 'STRIPE_LIVE' key is present and has a value of either 'true' or 'false'. All Stripe functionality is currently disabled.")
	}

	log.Fatal(http.ListenAndServe(":"+os.Getenv("PORT"), router))
}

func tokenToPaymentData(pre *Token) *PaymentData {
	return &PaymentData{
		Amount:      pre.Amount,
		Description: pre.Description,
		Name:        pre.Name,
		Addr1:       pre.Addr1,
		Addr2:       pre.Addr2,
		City:        pre.City,
		State:       pre.State,
		Zip:         pre.Zip,
		Email:       pre.Email,
		Phone:       pre.Phone,
	}
}

func sendPaymentEmail(data *PaymentData) {
	to := determineTeamEmail(data)
	if to != "" {
		username := os.Getenv("WEBSERVER_EMAIL_USERNAME")
		if username == "" {
			fmt.Println("ERROR: 'WEBSERVER_EMAIL_USERNAME' ENVIRONMENT VARIABLE UNAVAILABLE")
			return
		}
		password := os.Getenv("WEBSERVER_EMAIL_PASSWORD")
		if password == "" {
			fmt.Println("ERROR: 'WEBSERVER_EMAIL_PASSWORD' ENVIRONMENT VARIABLE UNAVAILABLE")
			return
		}
		serverAddress := os.Getenv("SmtpServerAddress")
		if serverAddress == "" {
			fmt.Println("ERROR: 'SmtpServerAddress' ENVIRONMENT VARIABLE UNAVAILABLE")
			return
		}
		serverPort := os.Getenv("SmtpServerPort")
		if serverPort == "" {
			fmt.Println("ERROR: 'serverPort' ENVIRONMENT VARIABLE UNAVAILABLE")
			return
		}
		body := "To: " + to + ", finance@pathfindersrobotics.org\r\nSubject: New Payment\r\n\r\nNew Payment\r\nAmount: " + strconv.FormatInt(data.Amount, 10) + "\r\nDescription: " + data.Description + "\r\nName: " + data.Name + "\r\nAddr1: " + data.Addr1 + "\r\nAddr2: " + data.Addr2 + "\r\nCity: " + data.City + "\r\nState: " + data.State + "\r\nZip: " + data.Zip + "\r\nEmail: " + data.Email + "\r\nPhone: " + data.Phone
		auth := smtp.PlainAuth("", username, password, serverAddress)
		err := smtp.SendMail(serverAddress+":"+serverPort, auth, username, []string{to, "finance@pathfindersrobotics.org"}, []byte(body))
		if err != nil {
			fmt.Println(err)
			fmt.Println("EMAIL COULD NOT BE SENT")
		}
	} else {
		fmt.Println("ERROR: TEAM EMAIL FROM DESCRIPTION INVALID")
	}
}

func determineTeamEmail(data *PaymentData) string {
	if strings.Contains(data.Description, "FTC Pathfinders 13497") {
		return "ftc13497@pathfindersrobotics.org"
	}
	if strings.Contains(data.Description, "FLL Pathfinders 7885") {
		return "fll7885@pathfindersrobotics.org"
	}
	return ""
}
