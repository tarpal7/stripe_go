package main

import (
	"encoding/json"
	"fmt"
	"github.com/joho/godotenv"
	stripe "github.com/stripe/stripe-go/v72"
	"github.com/stripe/stripe-go/v72/paymentintent"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
)

type CheckoutData struct {
	ClientSecret string `json:"client_secret"`
}

func main() {

	if err := godotenv.Load(); err != nil {
		log.Fatalf("error initializing configs: %s", err.Error())
	}
	//stripe.Key = "sk_test_51HyURQAgpXBKSE2oh2iOeS0hBsMuo3GHgTaqT5PtAy40AYd6DNt1psFlLWcnPMDAEYzZOULWGPjmVrqW1XcUxx8B00xxjF4LMm"
	secretKey, keyPresent := os.LookupEnv("STRIPE_SECRET_KEY")
	if !keyPresent {
		fmt.Fprintf(os.Stderr, "ERROR: Please set STRIPE_SECRET_KEY environment variable.\n")
		return
	}

	stripe.Key = secretKey
	f, err := os.OpenFile("successful_payments.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Println(err)
	}
	defer f.Close()
	logger := log.New(f, "payment_successful: ", log.LstdFlags)

	http.HandleFunc("/create-payment-intent", func(w http.ResponseWriter, r *http.Request) {

		params := &stripe.PaymentIntentParams{
			Amount:   stripe.Int64(2000),
			Currency: stripe.String(string(stripe.CurrencyUSD)),
			PaymentMethodTypes: []*string{
				stripe.String("card"),
			},
		}
		intent, _ := paymentintent.New(params)
		data := CheckoutData{
			ClientSecret: intent.ClientSecret,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(data)
	})

	http.HandleFunc("/webhook", func(w http.ResponseWriter, req *http.Request) {
		const MaxBodyBytes = int64(65536)
		req.Body = http.MaxBytesReader(w, req.Body, MaxBodyBytes)
		payload, err := ioutil.ReadAll(req.Body)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading request body: %v\n", err)
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}

		event := stripe.Event{}

		if err := json.Unmarshal(payload, &event); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to parse webhook body json: %v\n", err.Error())
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Unmarshal the event data into an appropriate struct depending on its Type
		switch event.Type {
		case "payment_intent.succeeded":
			var paymentIntent stripe.PaymentIntent
			err := json.Unmarshal(event.Data.Raw, &paymentIntent)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error parsing webhook JSON: %v\n", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			fmt.Println("PaymentIntent was successful! Amount: " + strconv.FormatInt(paymentIntent.Amount, 10))
		case "payment_method.attached":
			var paymentMethod stripe.PaymentMethod
			err := json.Unmarshal(event.Data.Raw, &paymentMethod)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error parsing webhook JSON: %v\n", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			fmt.Println("PaymentMethod was attached to a Customer!")
		case "charge.succeeded":
			var charge stripe.Charge
			err := json.Unmarshal(event.Data.Raw, &charge)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error parsing webhook JSON: %v\n", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			logMsg := "Charge succeeded! Customer:" + charge.BillingDetails.Name +
				"; Email:" + charge.BillingDetails.Email +
				"; Address:" + charge.BillingDetails.Address.Line1 + ", " +
				charge.BillingDetails.Address.Line2 + ", " +
				charge.BillingDetails.Address.City + ", " +
				charge.BillingDetails.Address.State + ", " +
				charge.BillingDetails.Address.PostalCode + ", " +
				charge.BillingDetails.Address.Country +
				"; Phone:" + charge.BillingDetails.Phone

			fmt.Println(logMsg)
			logger.Println(logMsg)
		// ... handle other event types
		default:
			fmt.Fprintf(os.Stderr, "Unhandled event type: %s\n", event.Type)
		}

		w.WriteHeader(http.StatusOK)
	})

	http.ListenAndServe(":8080", nil)
}
