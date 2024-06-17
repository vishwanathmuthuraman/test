package main

import (
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/sajari/regression"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type ShopifyOrderCount struct {
	Orders []struct {
		CurrentTotalPrice string    `json:"current_total_price"`
		CreatedAt         time.Time `json:"created_at"`
	} `json:"orders"`
}

type groupedOrders struct {
	Key              time.Time
	RevenueActual    float64
	RevenuePredicted float64
}

func GetOrderCounts(w http.ResponseWriter, r *http.Request, db *pgx.Conn) {
	sponsorId, err := strconv.Atoi(r.URL.Query().Get("sponsor_id"))
	if err != nil {
		http.Error(w, "Invalid sponsor id", http.StatusBadRequest)
		return
	}

	//account, err := routes.GetLogin(r, db)
	//if err != nil || account.Type != "sponsor" {
	//	//ErrorBack(w, r, err.Error(), 401)
	//
	//	http.Redirect(w, r, "/sponsor/login", 302)
	//	return
	//}
	//
	//if account.Id != sponsorId {
	//	http.Error(w, "You can only view your own data", http.StatusBadRequest)
	//	return
	//}

	var sponsorDomain string
	var sponsorToken string

	err = db.QueryRow(r.Context(), "SELECT shopify_domain, shopify_token FROM sponsor WHERE id = $1", sponsorId).Scan(&sponsorDomain, &sponsorToken)
	if err != nil {
		http.Error(w, "Can't look up sponsor", http.StatusBadRequest)
		return
	}

	startDateStr := r.URL.Query().Get("start_date")
	endDateStr := r.URL.Query().Get("end_date")

	startDateI, err := strconv.Atoi(startDateStr)
	if err != nil {
		http.Error(w, "Invalid start date format. Please use unix format.", http.StatusBadRequest)
		return
	}
	startDate := time.Unix(int64(startDateI/1000), 0)

	endDateI, err := strconv.Atoi(endDateStr)
	if err != nil {
		http.Error(w, "Invalid end date format. Please use unix format.", http.StatusBadRequest)
		return
	}
	endDate := time.Unix(int64(endDateI/1000), 0)

	// Validate that end date is after start date
	if endDate.Before(startDate) {
		http.Error(w, "End date should be after start date.", http.StatusBadRequest)
		return
	}

	//interval, err := strconv.Atoi(r.FormValue("interval"))
	//if err != nil {
	//	http.Error(w, "Invalid interval", http.StatusBadRequest)
	//	return
	//}
	interval := 1 * time.Hour

	// previous month to sample
	sampleStart := startDate.Add(-1 * time.Hour * 24 * 30)
	sampleEnd := startDate

	sample := make(map[time.Time]float64)
	actual, err := getOrderCountFromShopify(sampleStart, sampleEnd, sponsorDomain, sponsorToken)
	var endOfPeriod = sampleStart
	for endOfPeriod.Before(sampleEnd) {
		endOfPeriod = endOfPeriod.Add(interval)

		if err != nil {
			http.Error(w, fmt.Sprintf("Error calling Shopify API: %s", err.Error()), http.StatusInternalServerError)
			return
		}

		withinPeriod := 0.0
		for _, order := range actual.Orders {
			if order.CreatedAt.Before(endOfPeriod) && order.CreatedAt.After(endOfPeriod.Add(-interval)) {
				price, err := strconv.ParseFloat(order.CurrentTotalPrice, 64)
				if err == nil {
					withinPeriod += price
				}
			}
		}

		sample[endOfPeriod] = withinPeriod

	}

	// run the prediction
	times := make([]time.Time, len(sample))
	orderCountsValues := make([]float64, len(sample))
	for i, orderCount := range sample {
		//if i.Before(sampleEnd) {
		times = append(times, i)
		orderCountsValues = append(orderCountsValues, orderCount)
		//}
	}

	predictFor := make([]time.Time, 0)
	for i := startDate; i.Before(endDate); i = i.Add(time.Duration(interval)) {
		predictFor = append(predictFor, i)
	}

	predicted := PredictedValues(times, orderCountsValues, predictFor)
	res := make([]groupedOrders, 0)

	actual, err = getOrderCountFromShopify(startDate, endDate, sponsorDomain, sponsorToken)
	if err != nil {
		fmt.Println(err)
	}
	for i, predictedValue := range predicted {

		if err != nil {
			http.Error(w, fmt.Sprintf("Error calling Shopify API: %s", err.Error()), http.StatusInternalServerError)
			return
		}

		withinPeriod := 0.0
		for _, order := range actual.Orders {
			if order.CreatedAt.Before(i) && order.CreatedAt.After(i.Add(-interval)) {
				price, err := strconv.ParseFloat(order.CurrentTotalPrice, 64)
				if err == nil {
					withinPeriod += price
				} else {
					fmt.Println(err)
				}
			}
		}

		res = append(res, groupedOrders{
			Key:              i,
			RevenueActual:    withinPeriod,
			RevenuePredicted: float64(predictedValue),
		})

	}

	fmt.Println(res)
	// Encode the res map as JSON and write it to the response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}

func getOrderCountFromShopify(start, end time.Time, shopifyDomain string, apiAccessToken string) (ShopifyOrderCount, error) {
	nextLink := fmt.Sprintf("https://%s/admin/api/2023-10/orders.json?status=any&created_at_min=%s&created_at_max=%s&limit=250",
		shopifyDomain, start.Format(time.RFC3339), end.Format(time.RFC3339))

	// Create an HTTP client
	//var client = &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(&url.URL{
	//	Scheme: "http",
	//	User:   url.UserPassword("none", "none"),
	//	Host:   "34.29.250.56:3128",
	//})}}
	client := &http.Client{}

	var concatResponse ShopifyOrderCount = ShopifyOrderCount{}

	for nextLink != "" {
		// Create a GET request to the Shopify API
		req, err := http.NewRequest("GET", nextLink, nil)
		if err != nil {
			// Handle error
			fmt.Println("Error creating request:", err)
			return concatResponse, err
		}

		// Set the API access token in the request headers
		req.Header.Set("X-Shopify-Access-Token", apiAccessToken)

		// Send the request to the Shopify API
		resp, err := client.Do(req)
		if err != nil {
			// Handle error
			fmt.Println("Error sending request:", err)
			return concatResponse, err
		}
		defer resp.Body.Close()

		nextLinksString := resp.Header.Get("link")
		nextLinks := strings.Split(nextLinksString, ",")
		nextLinkTemp := ""
		for _, link := range nextLinks {
			var rgx = regexp.MustCompile(`\<(.*?)\>; rel="(.*?)"`)
			rs := rgx.FindStringSubmatch(link)
			if rs[2] == "next" {
				nextLinkTemp = rs[1]
			}
		}

		nextLink = nextLinkTemp

		// Parse the response body as JSON
		var res ShopifyOrderCount
		err = json.NewDecoder(resp.Body).Decode(&res)
		if err != nil {
			// Handle error
			fmt.Println("Error parsing response:", err)
			return concatResponse, err
		}

		concatResponse.Orders = append(concatResponse.Orders, res.Orders...)

	}

	return concatResponse, nil
}

func PredictedValues(sampleTimes []time.Time, sampleValues []float64, predictFor []time.Time) map[time.Time]float64 {
	r := new(regression.Regression)
	r.SetObserved("Sales")
	r.SetVar(0, "Date")
	r.SetVar(1, "Weekday")

	for i := 0; i < len(sampleTimes); i++ {
		r.Train(regression.DataPoint(float64(sampleValues[i]), []float64{float64(sampleTimes[i].Unix()), float64(sampleTimes[i].Weekday())}))
	}

	err := r.Run()
	if err != nil {
		fmt.Println(err)

		return nil
	}

	predictions := make(map[time.Time]float64)
	for _, predictTime := range predictFor {
		prediction, err := r.Predict([]float64{float64(predictTime.Unix()), float64(predictTime.Weekday())})
		if err != nil {
			fmt.Println(err)
			return nil
		}
		predictions[predictTime] = prediction
	}

	fmt.Printf("Regression formula:\n%v\n", r.Formula)
	fmt.Printf("Regression:\n%s\n", r)

	fmt.Println(predictions)
	return predictions
}
