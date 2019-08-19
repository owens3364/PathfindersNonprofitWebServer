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

// For creating Stripe payments via Credit Card
type PaymentData struct {
	Amount      *int    `form:"amount" json:"amount" binding:"exists"`
	Description *string `form:"description" json:"description" binding:"exists"`
	Name        *string `form:"name" json:"name" binding:"exists"`
	Addr1       *string `form:"addr1" json:"addr1" binding:"exists"`
	Addr2       *string `form:"addr2" json:"addr2" binding:"exists"`
	City        *string `form:"city" json:"city" binding:"exists"`
	State       *string `form:"state" json:"state" binding:"exists"`
	Zip         *string `form:"zip" json:"zip" binding:"exists"`
	Email       *string `form:"email" json:"email" binding:"exists"`
	Phone       *string `form:"phone" json:"phone" binding:"exists"`
}

// For creating Stripe payments via PaymentRequestButton
type Token struct {
	Amount      *int    `form:"amount" json:"amount" binding:"exists"`
	Description *string `form:"description" json:"description" binding:"exists"`
	Name        *string `form:"name" json:"name" binding:"exists"`
	Addr1       *string `form:"addr1" json:"addr1" binding:"exists"`
	Addr2       *string `form:"addr2" json:"addr2" binding:"exists"`
	City        *string `form:"city" json:"city" binding:"exists"`
	State       *string `form:"state" json:"state" binding:"exists"`
	Zip         *string `form:"zip" json:"zip" binding:"exists"`
	Email       *string `form:"email" json:"email" binding:"exists"`
	Phone       *string `form:"phone" json:"phone" binding:"exists"`
	StripeToken *string `form:"token" json:"token" binding:"exists"`
}

type EmailData struct {
	DonorInformation PaymentData

	TeamEmail                string
	Team                     string
	FIRSTSuffix              string
	PRAddr1                  string
	PRCity                   string
	PRState                  string
	PRZip                    string
	PRPhone                  string
	EIN                      string
	Date                     string
	CurrentSeason            string
	WebServerEmail           string
	WebServerPassword        string
	DonationReceiptsEmail    string
	DonationReceiptsPassword string
	ServerAddress            string
	ServerPort               string
}

type osEnvVarError struct {
	errorMessage string
}

func (errType *osEnvVarError) Error() string {
	return errType.errorMessage
}

const FTCPathfinders13497 string = "FTC Pathfinders 13497"
const FLLPhoenixVoyagers7885 string = "FLL Phoenix Voyagers 7885"

const Email13497 string = "ftc13497@pathfindersrobotics.org"
const Email7885 string = "fll7885@pathfindersrobotics.org"

const FTCSuffix string = "Tech Challenge"
const FLLSuffix string = "Lego League"

const EmailFinance string = "finance@pathfindersrobotics.org"

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
			c.Header("X-Frame-Options", "allow-from https://js.stripe.com")
			c.Header("X-XSS-Protection", "1; mode=block")
			c.Header("Content-Security-Policy", "default-src 'none'; script-src 'self' https://storage.googleapis.com https://www.google-analytics.com https://s.ytimg.com https://www.youtube.com https://js.stripe.com https://ajax.cloudflare.com 'sha256-5As4+3YpY62+l38PsxCEkjB1R4YtyktBtRScTJ3fyLU='; style-src 'self' https://maxcdn.bootstrapcdn.com 'sha256-oWyvTH6ZfCvIDieRREt+hfVBWcf5gzk2WAW4xELF74Q='; img-src 'self' https://www.google-analytics.com data:; connect-src 'self' https://js.stripe.com https://www.google-analytics.com https://www.youtube.com https://m.stripe.network https://q.stripe.com; media-src https://www.youtube.com; child-src 'self' https://www.youtube.com https://js.stripe.com https://m.stripe.network; form-action 'self'; worker-src 'self';")
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
			txt := string([]rune(path)[0:7])
			if path == "/" || txt == "/static" || txt == "/assets" {
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
				err := c.BindJSON(&data)
				if err != nil {
					fmt.Println(err)
				}
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
			err := c.BindJSON(&paymentIntentData)
			if err != nil {
				fmt.Println(err)
			}
			intent, _ := paymentintent.New(&stripe.PaymentIntentParams{
				Amount:      stripe.Int64(int64(*paymentIntentData.Amount)),
				Currency:    stripe.String(string(stripe.CurrencyUSD)),
				Description: stripe.String(*paymentIntentData.Description),
				PaymentMethodTypes: []*string{
					cardPointer,
				},
				ReceiptEmail: stripe.String(*paymentIntentData.Email),
				Shipping: &stripe.ShippingDetailsParams{
					Address: &stripe.AddressParams{
						City:       stripe.String(*paymentIntentData.City),
						Country:    stripe.String("US"),
						Line1:      stripe.String(*paymentIntentData.Addr1),
						Line2:      stripe.String(*paymentIntentData.Addr2),
						PostalCode: stripe.String(*paymentIntentData.Zip),
						State:      stripe.String(*paymentIntentData.State),
					},
					Name:  stripe.String(*paymentIntentData.Name),
					Phone: stripe.String(*paymentIntentData.Phone),
				},
			})
			c.JSON(200, gin.H{
				"secret": intent.ClientSecret,
			})
		})

		router.POST("/paymentRequest", func(c *gin.Context) {
			var token Token
			err := c.BindJSON(&token)
			if err != nil {
				fmt.Println(err)
			}

			params := &stripe.ChargeParams{
				Amount:       stripe.Int64(int64(*token.Amount)),
				Currency:     stripe.String(string(stripe.CurrencyUSD)),
				Description:  stripe.String(*token.Description),
				ReceiptEmail: stripe.String(*token.Email),
				Shipping: &stripe.ShippingDetailsParams{
					Address: &stripe.AddressParams{
						City:       stripe.String(*token.City),
						Country:    stripe.String("US"),
						Line1:      stripe.String(*token.Addr1),
						Line2:      stripe.String(*token.Addr2),
						PostalCode: stripe.String(*token.Zip),
						State:      stripe.String(*token.State),
					},
					Name:  token.Name,
					Phone: token.Phone,
				},
				Source: &stripe.SourceParams{
					Token: token.StripeToken,
				},
			}
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
	htmlEmail := "<html xmlns=\"http://www.w3.org/1999/xhtml\"><head><meta http-equiv=\"Content-Type\" content=\"text/html; charset=UTF-8\" /><title>Pathfinders Robotics Donation Receipt</title><meta name=\"viewport\" content=\"width=device-width, initial-scale=1.0\"/><meta http-equiv=\"X-UA-Compatible\" content=\"IE=7\" /><meta http-equiv=\"X-UA-Compatible\" content=\"IE=8\" /><meta http-equiv=\"X-UA-Compatible\" content=\"IE=9\" /><!--[if !mso]><!-- --><meta http-equiv=\"X-UA-Compatible\" content=\"IE=edge\" /><!--<![endif]--></head><body style=\"margin: 0; padding: 0; font-family: 'Times New Roman', Times, serif; letter-spacing: 0em;\"><table border=\"0\" cellpadding=\"0\" cellspacing=\"0\" width=\"100%\"  style=\"font-size: 12pt;\"><tr><td style=\"width: 50%;\"><img src=\"https://www.pathfindersrobotics.org/assets/receipts/Logo.png\" alt=\"Pathfinders Robotics\" width=\"265\" border=\"0\" style=\"display: block; height: auto;\" /></td><td style=\"width: 50%; text-align: right;\">Pathfinders Robotics<br/>${PRAddr1}, ${PRCity}, ${PRState} ${PRZip}<br/>${PRPhone}&nbsp;</td></tr></table><table border=\"0\" cellpadding=\"0\" cellspacing=\"0\" width=\"100%\" style=\"border-bottom: 2px solid black; font-size: 14pt;\"><tr><td>${Date}<br/><br/>${Name}<br/>${Addr1}${Addr2}<br/>${City}, ${State} ${Zip}<br/><br/>Thank you so much for your very generous donation of $${Amount} to the Pathfinders Robotics organization received on ${Date}.<br/><br/>Your donation will help us in supporting ${Team} in FIRST® ${FIRSTSuffix}. We will proudly include ${Name} information and branding during the ${CurrentSeason} FIRST® ${FIRSTSuffix} season.<br/><br/>Thanks again for your generosity and support.<br/><br/>Respectfully,<img src=\"https://www.pathfindersrobotics.org/assets/receipts/Signature.png\" alt=\"Bhooshan Karnik\" width=\"160\" border=\"0\" style=\"display: block; height: auto;\" /><br/>Bhooshan Karnik<br/>Treasurer of Pathfinders Robotics<br/><br/><br/></td></tr></table><br/><table border=\"0\" cellpadding=\"0\" cellspacing=\"0\" width=\"100%\"><tr><td style=\"text-align: center; font-size: 11pt;\"><b>Donation receipt</b> - Keep for your records</td></tr><tr><td style=\"font-size: 13pt;\">Donor: ${Name}<br/>Date Received: ${Date}<br/>Cash Contribution: $${Amount}<br/><br/>Pathfinders Robotics<br/>${PRAddr1}<br/>${PRCity}, ${PRState} ${PRZip}<br/>Federal Tax ID ${EIN}</td></tr></table></body></html>"

	emailData, err := genEmailData(*data)
	if err == nil {
		notifBody := "To: " + emailData.TeamEmail + "\r\nSubject: New Payment\r\n\r\nNew Payment\r\nAmount: " + strconv.Itoa(*data.Amount) + "\r\nDescription: " + *data.Description + "\r\nName: " + *data.Name + "\r\nAddr1: " + *data.Addr1 + "\r\nAddr2: " + *data.Addr2 + "\r\nCity: " + *data.City + "\r\nState: " + *data.State + "\r\nZip: " + *data.Zip + "\r\nEmail: " + *data.Email + "\r\nPhone: " + *data.Phone
		notifAuth := smtp.PlainAuth("", emailData.WebServerEmail, emailData.WebServerPassword, emailData.ServerAddress)
		notifErr := smtp.SendMail(emailData.ServerAddress+":"+emailData.ServerPort, notifAuth, emailData.WebServerEmail, []string{emailData.TeamEmail, EmailFinance}, []byte(notifBody))
		if notifErr != nil {
			fmt.Println(notifErr)
			fmt.Println("ERROR: NOTIFICATION EMAIL TO TEAM AND FINANCE (BCC) COULD NOT BE SENT")
		}

		htmlEmail = strings.ReplaceAll(htmlEmail, "${PRAddr1}", emailData.PRAddr1)
		htmlEmail = strings.ReplaceAll(htmlEmail, "${PRCity}", emailData.PRCity)
		htmlEmail = strings.ReplaceAll(htmlEmail, "${PRState}", emailData.PRState)
		htmlEmail = strings.ReplaceAll(htmlEmail, "${PRZip}", emailData.PRZip)
		htmlEmail = strings.ReplaceAll(htmlEmail, "${PRPhone}", emailData.PRPhone)
		htmlEmail = strings.ReplaceAll(htmlEmail, "${Date}", emailData.Date)
		htmlEmail = strings.ReplaceAll(htmlEmail, "${Name}", *emailData.DonorInformation.Name)
		htmlEmail = strings.ReplaceAll(htmlEmail, "${Addr1}", *emailData.DonorInformation.Addr1)
		htmlEmail = strings.ReplaceAll(htmlEmail, "${Addr2}", *emailData.DonorInformation.Addr2)
		htmlEmail = strings.ReplaceAll(htmlEmail, "${City}", *emailData.DonorInformation.City)
		htmlEmail = strings.ReplaceAll(htmlEmail, "${State}", *emailData.DonorInformation.State)
		htmlEmail = strings.ReplaceAll(htmlEmail, "${Zip}", *emailData.DonorInformation.Zip)
		htmlEmail = strings.ReplaceAll(htmlEmail, "${Amount}", fmt.Sprintf("%g", float64(*emailData.DonorInformation.Amount)/100.0))
		htmlEmail = strings.ReplaceAll(htmlEmail, "${Team}", emailData.Team)
		htmlEmail = strings.ReplaceAll(htmlEmail, "${FIRSTSuffix}", emailData.FIRSTSuffix)
		htmlEmail = strings.ReplaceAll(htmlEmail, "${CurrentSeason}", emailData.CurrentSeason)
		htmlEmail = strings.ReplaceAll(htmlEmail, "${EIN}", emailData.EIN)

		receiptBody := "To: " + emailData.DonationReceiptsEmail + "\r\nSubject: Pathfinders Robotics Donation Receipt\r\n" + "MIME-version: 1.0;\nContent-Type: text/plain; charset=\"UTF-8\";\n\n" + htmlEmail
		receiptAuth := smtp.PlainAuth("", emailData.DonationReceiptsEmail, emailData.DonationReceiptsPassword, emailData.ServerAddress)
		receiptErr := smtp.SendMail(emailData.ServerAddress+":"+emailData.ServerPort, receiptAuth, emailData.DonationReceiptsEmail, []string{*emailData.DonorInformation.Email, emailData.TeamEmail, EmailFinance}, []byte(receiptBody))
		if receiptErr != nil {
			fmt.Println(receiptErr)
			fmt.Println("ERROR: RECEIPT EMAIL TO DONOR AND TEAM (BCC) AND FINANCE(BCC) COULD NOT BE SENT")
		}
	} else {
		fmt.Println("ERROR: EMAIL COULD NOT BE GENERATED OR DELIVERED")
	}
}

func genEmailData(origin PaymentData) (EmailData, error) {
	teamEmail, team, firstSuffix := determineTeamEmail(&origin)

	if teamEmail == "" || team == "" || firstSuffix == "" {
		err := "ERROR: FIELD 'DESCRIPTION' IN TYPE 'PaymentData' NOT RECOGNIZED"
		fmt.Println(err)
		return EmailData{}, &osEnvVarError{err}
	}

	prAddr1 := os.Getenv("PRAddr1")
	if prAddr1 == "" {
		err := "ERROR: 'PRAddr1' ENVIRONMENT VARIABLE UNAVAILABLE"
		fmt.Println(err)
		return EmailData{}, &osEnvVarError{err}
	}
	prCity := os.Getenv("PRCity")
	if prCity == "" {
		err := "ERROR: 'PRCity' ENVIRONMENT VARIABLE UNAVAILABLE"
		fmt.Println(err)
		return EmailData{}, &osEnvVarError{err}
	}
	prState := os.Getenv("PRState")
	if prState == "" {
		err := "ERROR: 'PRState' ENVIRONMENT VARIABLE UNAVAILABLE"
		fmt.Println(err)
		return EmailData{}, &osEnvVarError{err}
	}
	prZip := os.Getenv("PRZip")
	if prZip == "" {
		err := "ERROR: 'PRZip' ENVIRONMENT VARIABLE UNAVAILABLE"
		fmt.Println(err)
		return EmailData{}, &osEnvVarError{err}
	}
	prPhone := os.Getenv("PRPhone")
	if prPhone == "" {
		err := "ERROR: 'PRPhone' ENVIRONMENT VARIABLE UNAVAILABLE"
		fmt.Println(err)
		return EmailData{}, &osEnvVarError{err}
	}

	ein := os.Getenv("EIN")
	if ein == "" {
		err := "ERROR: 'EIN' ENVIRONMENT VARIABLE UNAVAILABLE"
		fmt.Println(err)
		return EmailData{}, &osEnvVarError{err}
	}

	est, _ := time.LoadLocation("EST")
	currentTime := time.Now().In(est)
	month := currentTime.Month().String()
	day := currentTime.Day()
	year := currentTime.Year()
	date := month + " " + strconv.Itoa(day) + ", " + strconv.Itoa(year)

	currentSeason := ""
	if currentTime.Month() < 5 {
		currentSeason = strconv.Itoa(year-1) + "-" + strconv.Itoa(year)
	} else {
		currentSeason = strconv.Itoa(year) + "-" + strconv.Itoa(year+1)
	}

	webserverUsr := os.Getenv("WEBSERVER_EMAIL_USERNAME")
	if webserverUsr == "" {
		err := "ERROR: 'WEBSERVER_EMAIL_USERNAME' ENVIRONMENT VARIABLE UNAVAILABLE"
		fmt.Println(err)
		return EmailData{}, &osEnvVarError{err}
	}
	webserverPwd := os.Getenv("WEBSERVER_EMAIL_PASSWORD")
	if webserverPwd == "" {
		err := "ERROR: 'WEBSERVER_EMAIL_PASSWORD' ENVIRONMENT VARIABLE UNAVAILABLE"
		fmt.Println(err)
		return EmailData{}, &osEnvVarError{err}
	}

	donationReceiptsUsr := os.Getenv("DONATION_RECEIPTS_EMAIL_USERNAME")
	if donationReceiptsUsr == "" {
		err := "ERROR: 'DONATION_RECEIPTS_EMAIL_USERNAME' ENVIRONMENT VARIABLE UNAVAILABLE"
		fmt.Println(err)
		return EmailData{}, &osEnvVarError{err}
	}
	donationReceiptsPwd := os.Getenv("DONATION_RECEIPTS_EMAIL_PASSWORD")
	if donationReceiptsPwd == "" {
		err := "ERROR: 'DONATION_RECEIPTS_EMAIL_PASSWORD' ENVIRONMENT VARIABLE UNAVAILABLE"
		fmt.Println(err)
		return EmailData{}, &osEnvVarError{err}
	}

	serverAddress := os.Getenv("SmtpServerAddress")
	if serverAddress == "" {
		err := "ERROR: 'SmtpServerAddress' ENVIRONMENT VARIABLE UNAVAILABLE"
		fmt.Println(err)
		return EmailData{}, &osEnvVarError{err}
	}
	serverPort := os.Getenv("SmtpServerPort")
	if serverPort == "" {
		err := "ERROR: 'serverPort' ENVIRONMENT VARIABLE UNAVAILABLE"
		fmt.Println(err)
		return EmailData{}, &osEnvVarError{err}
	}

	return EmailData{
		DonorInformation:         origin,
		TeamEmail:                teamEmail,
		Team:                     team,
		FIRSTSuffix:              firstSuffix,
		PRAddr1:                  prAddr1,
		PRCity:                   prCity,
		PRState:                  prState,
		PRZip:                    prZip,
		PRPhone:                  prPhone,
		EIN:                      ein,
		Date:                     date,
		CurrentSeason:            currentSeason,
		WebServerEmail:           webserverUsr,
		WebServerPassword:        webserverPwd,
		DonationReceiptsEmail:    donationReceiptsUsr,
		DonationReceiptsPassword: donationReceiptsPwd,
		ServerAddress:            serverAddress,
		ServerPort:               serverPort,
	}, nil
}

func determineTeamEmail(data *PaymentData) (string, string, string) {
	if strings.Contains(*data.Description, FTCPathfinders13497) {
		return Email13497, FTCPathfinders13497, FTCSuffix
	}
	if strings.Contains(*data.Description, FLLPhoenixVoyagers7885) {
		return Email7885, FLLPhoenixVoyagers7885, FLLSuffix
	}
	return "", "", ""
}
