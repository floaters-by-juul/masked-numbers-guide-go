package main

import (
	"fmt"
	"log"
	"net/http"
	"strconv"

	messagebird "github.com/messagebird/go-rest-api"
)

// landing handler is the default view
// loads database int dbdata struct and displays the default view
func landing(dbdata *RideSharingDB) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		err := dbdata.loadDB()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "Server encountered an error: %v", err)
			return
		}
		renderDefaultTemplate(w, "views/landing.gohtml", dbdata)
		return
	}
}

// createRideHandler returns a handler that:
// - loads database into dbdata struct
// - checks proxy numbers that are not already in use
// - parses POST requests submitted to this route for new ride
// - Prepares and executes a SQL statement for the new ride, inserting ride data
// - sends an sms notification to the customer and driver for that ride
// - reloads database and updates view
func createRideHandler(dbdata *RideSharingDB, mb *messagebird.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := dbdata.loadDB()
		if err != nil {
			log.Println(err)
			dbdata.Message = fmt.Sprint(err)
			renderDefaultTemplate(w, "views/landing.gohtml", dbdata)
			return
		}

		if r.Method == "POST" {
			r.ParseForm()
			customerID := r.FormValue("customer")
			driverID := r.FormValue("driver")
			startLocation := r.FormValue("start")
			destinationLocation := r.FormValue("destination")
			dateTime := r.FormValue("datetime")

			// Convert ids from form values to ints which are used in our data model
			// Also to prepare to send SMS notifications to customer and driver for new ride
			customerIDint, err := strconv.Atoi(customerID)
			driverIDint, err := strconv.Atoi(driverID)
			if err != nil {
				dbdata.Message = fmt.Sprintf("Something went wrong: %v", err)
				renderDefaultTemplate(w, "views/landing.gohtml", dbdata)
				return
			}

			// Check for an available proxy number
			availableProxy, err := getAvailableProxyNumber(dbdata, customerIDint, driverIDint)
			if err != nil {
				dbdata.Message = fmt.Sprintf("We encountered an error: %v", err)
				log.Println(err)
				renderDefaultTemplate(w, "views/landing.gohtml", dbdata)
				return
			}

			// Prepare SQL statement for new ride entry and insert into database
			q := fmt.Sprintf(
				"INSERT INTO rides (start,destination,datetime,customer_id,driver_id,number_id) VALUES ('%s','%s','%s','%s','%s','%d')",
				startLocation,
				destinationLocation,
				dateTime,
				customerID,
				driverID,
				availableProxy.ID,
			)
			query := []string{q}
			dbInsert(query)

			// Notify this customer
			mbSender(
				mb,
				availableProxy.Number,
				[]string{dbdata.Customers[customerIDint].Number},
				fmt.Sprintf("%s will pick you up at %s. Reply to this message to contact the driver.", dbdata.Drivers[driverIDint].Name, dateTime),
				nil,
			)

			// Notify this driver
			mbSender(
				mb,
				availableProxy.Number,
				[]string{dbdata.Drivers[driverIDint].Number},
				fmt.Sprintf("%s will pick you up at %s. Reply to this message to contact the driver.", dbdata.Customers[customerIDint].Name, dateTime),
				nil,
			)
		}

		// Re-load db just before we render the page
		err = dbdata.loadDB()
		if err != nil {
			log.Println(err)
			dbdata.Message = fmt.Sprint(err)
			renderDefaultTemplate(w, "views/landing.gohtml", dbdata)
			return
		}

		renderDefaultTemplate(w, "views/landing.gohtml", dbdata)
		return
	}
}

/* This is the shape of the r.Form submitted when MessageBird forwards an SMS as a POST request to a URL.
map[message_id:[7a76afeaef3743d28d0e2d93621235ca] originator:[16132093477] reference:[47749346971] createdDatetime:[2018-09-24T08:30:59+00:00] id:[f91908b75f9e4b1fba3b96dc44995f03] message:[this is a test message] receiver:[14708000894] body:[this is a test message] date:[1537806659] payload:[this is a test message] sender:[16132093477] date_utc:[1537777859] recipient:[14708000894]]
*/

// messageHookHandler handles POST requests forwarded by the MessageBird REST API to our application
// This handler:
// - Loads the database into dbdata struct
// - Checks if we're receiving a POST request
// - If we're receiving a post request,
// -- Loop through rides in dbdata and checks if we're receiving this message from a valid proxy number.
// -- If proxy number is valid, check sender is a customer or driver
// -- If proxy number is not valid, display an error
// -- If we can't find the sender in our customer or driver database, display an error
func messageHookHandler(dbdata *RideSharingDB, mb *messagebird.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := dbdata.loadDB()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "Server encountered an error: %v", err)
			return
		}

		if r.Method == "POST" {
			// Read response from MessageBird REST API servers
			r.ParseForm()
			originator := r.FormValue("originator")
			receiver := r.FormValue("receiver")
			payload := r.FormValue("payload")

			// Check rides for proxy number used
			// Proxy number should be unique in list of rides
			for _, v := range dbdata.Rides {
				if v.ThisProxyNumber.Number == receiver {
					switch {
					case checkIfCustomer(dbdata, originator):
						// forward message to driver
						mbSender(
							mb,
							receiver,
							[]string{v.ThisDriver.Number},
							payload,
							nil,
						)
						return
					case checkIfDriver(dbdata, originator):
						// forward message to customer
						mbSender(
							mb,
							receiver,
							[]string{v.ThisCustomer.Number},
							payload,
							nil,
						)
						return
					default:
						log.Printf("Could not find ride for customer/driver %s that uses proxy %s", originator, receiver)
					}
				} else {
					log.Printf("Unknown proxy number: %s", receiver)
				}
			}
			// Return any response, MessageBird won't parse this
			fmt.Fprint(w, "OK")
			return
		}
	}
}

/* This is the shape of the r.Form submitted when MessageBird forwards a call as a GET request to a URL.
map[callID:[2894efe1-63b7-4d37-b006-3aab7fcd4d49] destination:[14708000894] numberID:[272cca7c-c2d6-4781-9e92-168ba0520639] source:[Restricted] variables:[{}]]
*/

// voiceHookHandler handles GET requests forwarded from the MessageBird Server to our application
// This handler:
// - Writes only XML as output -- specifically, we are returning call flows written in XML
// - load database into dbdata struct
// - Parse form data submitted via GET request
// - Check rides for proxy number being called by caller
// - Check if caller is a customer or driver, and load the appropriate number to forward the call to
// - If we can't find the proxy number, customer number, or driver number, answer the call with message that call has failed
// - If we successfully find the customer or driver number, forward the call to that number.
func voiceHookHandler(dbdata *RideSharingDB, mb *messagebird.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// XML-only response
		w.Header().Set("Content-Type", "application/xml")

		err := dbdata.loadDB()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "Server encountered an error: %v", err)
			return
		}

		r.ParseForm()
		proxyNumber := r.FormValue("destination")
		caller := r.FormValue("source")

		var forwardToThisNumber string

		transactionFailXML := fmt.Sprint("<?xml version='1.0' encoding='UTF-8'?>" +
			"<Say language='en-GB' voice='female'>Sorry, we cannot identify your transaction. " +
			"Please make sure you have call in from the number you registered.</Say><Hangup />")

		for _, v := range dbdata.Rides {
			if v.ThisProxyNumber.Number == proxyNumber {
				switch {
				case checkIfCustomer(dbdata, caller):
					// Forward call to driver
					forwardToThisNumber = v.ThisDriver.Number
				case checkIfDriver(dbdata, caller):
					// Forward call to customer
					forwardToThisNumber = v.ThisCustomer.Number
				default:
					// Speaks transaction fail message and returns
					fmt.Fprint(w, transactionFailXML)
					log.Printf("Transfer to %s failed.", forwardToThisNumber)
					return
				}
			} else {
				// Speaks transaction fail message and returns
				fmt.Fprint(w, transactionFailXML)
				log.Printf("Transfer to %s failed.", forwardToThisNumber)
				return
			}
		}
		// If we get to this point, assume all is in order and attempt to transfer the call
		log.Println("Transferring call to ", forwardToThisNumber)
		fmt.Fprintf(w, "<?xml version='1.0' encoding='UTF-8'?><Transfer destination='%s' make='true' />", forwardToThisNumber)
		return
	}
}
