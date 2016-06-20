package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/mux"
)

const PROD_URL = "https://api.instamojo.com"
const TEST_URL = "https://test.instamojo.com"

var (
	prodClientID     *string
	prodClientSecret *string
	testClientID     *string
	testClientSecret *string
)

func main() {
	log.SetFlags(log.Lshortfile)
	prodClientID = flag.String("production-client-id", "", "Production Client ID")
	prodClientSecret = flag.String("production-client-secret", "", "Production Client Secret")
	testClientID = flag.String("test-client-id", "", "Test Client ID")
	testClientSecret = flag.String("test-client-secret", "", "Test Client Secret")
	flag.Parse()

	if *prodClientID == "" {
		log.Fatal("Production Client ID is missing")
	}

	if *prodClientSecret == "" {
		log.Fatal("Production Client secret is missing")
	}

	if *testClientID == "" {
		log.Fatal("Test Client ID is missing")
	}

	if *testClientSecret == "" {
		log.Fatal("Test Client Secret is missing")
	}

	router := mux.NewRouter()
	router.HandleFunc("/create", createOrderTokens).Methods("GET")
	router.HandleFunc("/status", statusHandler).Methods("GET")
	router.HandleFunc("/refund", refundHandler).Methods("POST")

	log.Fatal(http.ListenAndServe(":8080", LoggingHandler(router)))
}

func createOrderTokens(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	env := r.FormValue("env")
	authUrl := PROD_URL
	id := *prodClientID
	secret := *prodClientSecret
	if env == "test" {
		authUrl = TEST_URL
		id = *testClientID
		secret = *testClientSecret
	}

	authUrl += "/oauth2/token/"
	values := url.Values{}
	values.Set("client_id", id)
	values.Set("client_secret", secret)
	values.Set("grant_type", "client_credentials")
	authRequest, err := http.NewRequest("POST", authUrl, bytes.NewBufferString(values.Encode()))
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	authRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := &http.Client{}
	resp, err := client.Do(authRequest)
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var jsonResponse struct {
		AccessToken   string `json:"access_token,omitempty"`
		Error         string `json:"error,omitempty"`
		TransactionID string `json:"transaction_id,omitempty"`
	}

	if err = json.Unmarshal(data, &jsonResponse); err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if jsonResponse.AccessToken != "" {
		jsonResponse.TransactionID = generateRandomString(15)
	}

	marshalledData, err := json.Marshal(jsonResponse)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(marshalledData)
}

func statusHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	env := r.FormValue("env")
	orderID := r.FormValue("order_id")
	transactionID := r.FormValue("transaction_id")

	statusURL := PROD_URL
	if env == "test" {
		statusURL = TEST_URL
	}

	statusURL += "/v2/gateway/orders/"
	if orderID == "" {
		statusURL += "transaction_id:" + transactionID + "/"
	} else {
		statusURL += "id:" + orderID + "/"
	}

	statusRequest, err := http.NewRequest("GET", statusURL, nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	statusRequest.Header.Set("Authorization", r.Header.Get("Authorization"))
	statusRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	resp, err := client.Do(statusRequest)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	data, err := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if resp.StatusCode != http.StatusOK {
		w.WriteHeader(resp.StatusCode)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

func refundHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	env := r.Form.Get("env")
	paymentID := r.Form.Get("payment_id")
	amount := r.Form.Get("amount")

	refundURL := PROD_URL
	if env == "test" {
		refundURL = TEST_URL
	}

	refundURL += fmt.Sprintf("/v2/payments/%s/refund/", paymentID)
	params := url.Values{}
	params.Set("type", "PTH")
	params.Set("refund_amount", amount)
	params.Set("body", "I want my moneeeyyyyyyy")

	refundRequest, err := http.NewRequest("POST", refundURL, bytes.NewBufferString(params.Encode()))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	refundRequest.Header.Set("Authorization", r.Header.Get("Authorization"))
	refundRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	resp, err := client.Do(refundRequest)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	_, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(resp.StatusCode)

}

func generateRandomString(strlen int) string {
	rand.Seed(time.Now().UTC().UnixNano())
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	result := make([]byte, strlen)
	for i := 0; i < strlen; i++ {
		result[i] = chars[rand.Intn(len(chars))]
	}
	return string(result)
}