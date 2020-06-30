package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"reflect"

	messagebird "github.com/messagebird/go-rest-api"
	"github.com/messagebird/go-rest-api/sms"
)

// Helpers

func renderDefaultTemplate(w http.ResponseWriter, thisView string, data interface{}) {
	renderthis := []string{thisView, "views/layouts/default.gohtml"}
	t, err := template.ParseFiles(renderthis...)
	if err != nil {
		log.Fatal(err)
	}
	err = t.ExecuteTemplate(w, "default", data)
	if err != nil {
		log.Fatal(err)
	}
}

// getAvailableProxyNumber returns the a proxy number not already part of
// a customer+proxy && driver+proxy combination
func getAvailableProxyNumber(dbdata *RideSharingDB, customerID int, driverID int) (ProxyNumberType, error) {
	// If no rides, then return a random Proxy Number.
	if len(dbdata.Rides) == 0 {
		// Because Go doesn't read maps in sequence, we can use a for loop to select a random number
		for _, v := range dbdata.ProxyNumbers {
			return v, nil
		}
		// If we're here, then we've failed to get a proxy number; return error
		return (ProxyNumberType{}), fmt.Errorf("no available proxy numbers")
	}

	// Checks if []int contains an int
	containsNumGrp := func(arr [][]int, findme []int) bool {
		for _, v := range arr {
			if reflect.DeepEqual(v, findme) {
				return true
			}
		}
		return false
	}

	// rideProxySets is a slice of sets (also a slice) of proxy numbers,
	// e.g. []int{customerID,proxyNumber} or []int{driverID,proxyNumber}
	// These sets must be unique in order for our number masking system to work
	var rideProxySets [][]int
	for _, v1 := range dbdata.Rides {
		rideProxySets = append(rideProxySets, v1.NumGrp...)
	}

	// Once we have a list of all proxy sets that exist,
	// we iterate through our list of proxy numbers and
	// check if sets formed by the current POST request (passed into this function)
	// can form a proxy set that does not exist yet.
	for _, v2 := range dbdata.ProxyNumbers {
		// Check if both customer/driver+proxy number sets do not exist in current proxy sets
		if !containsNumGrp(rideProxySets, []int{customerID, v2.ID}) && !containsNumGrp(rideProxySets, []int{driverID, v2.ID}) {
			return v2, nil
		}
	}

	// If we end up here, then we've failed to get a proxy number
	return (ProxyNumberType{}), fmt.Errorf("no available proxy numbers")
}

func checkIfCustomer(dbdata *RideSharingDB, checkme string) bool {
	for _, v := range dbdata.Customers {
		if v.Number == checkme {
			return true
		}
	}
	return false
}

func checkIfDriver(dbdata *RideSharingDB, checkme string) bool {
	for _, v := range dbdata.Drivers {
		if v.Number == checkme {
			return true
		}
	}
	return false
}

// mbError handles MessageBird REST API errors
func mbError(err error) {
	if err != nil {
		switch errResp := err.(type) {
		case messagebird.ErrorResponse:
			for _, mbError := range errResp.Errors {
				log.Printf("Error: %#v\n", mbError)
			}
		}

		return
	}
}

// mbSender sends SMS messages
func mbSender(mb *messagebird.Client, originator string, recipient []string, msgbody string, params *sms.Params) {
	msg, err := sms.Create(
		mb,
		originator,
		recipient,
		msgbody,
		params,
	)
	if err != nil {
		mbError(err)
		log.Printf("Could not send sms notification to %s", recipient)
	} else {
		log.Print(msg)
	}
}
