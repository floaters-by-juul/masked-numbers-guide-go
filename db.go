package main

import (
	"database/sql"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func dbInsert(queries []string) {
	db, err := sql.Open("sqlite3", "./ridesharing.db")
	must(err)
	for _, i := range queries {
		statement, err := db.Prepare(i)
		must(err)
		_, err = statement.Exec()
		must(err)
	}
	defer db.Close()
}

// initExampleDB inserts example data into the sqlite db
func initExampleDB() {
	createTables := []string{
		"CREATE TABLE IF NOT EXISTS customers(id INTEGER PRIMARY KEY, name TEXT, number TEXT UNIQUE)",
		"CREATE TABLE IF NOT EXISTS drivers (id INTEGER PRIMARY KEY, name TEXT, number TEXT UNIQUE)",
		"CREATE TABLE IF NOT EXISTS proxy_numbers (id INTEGER PRIMARY KEY, number TEXT UNIQUE)",
		"CREATE TABLE IF NOT EXISTS " +
			"rides (id INTEGER PRIMARY KEY, " +
			"start TEXT, destination TEXT, datetime TEXT, customer_id INTEGER, driver_id INTEGER, number_id INTEGER, " +
			"FOREIGN KEY (customer_id) REFERENCES customers(id), FOREIGN KEY (driver_id) REFERENCES drivers(id))",
	}
	dbInsert(createTables)
	insertData := []string{
		"INSERT INTO customers (name, number) VALUES ('Caitlyn Carless', '319700000') ON CONFLICT (number) DO UPDATE SET name=excluded.name",
		"INSERT INTO customers (name, number) VALUES ('Danny Bikes', '319700001') ON CONFLICT (number) DO UPDATE SET name=excluded.name",
		"INSERT INTO drivers (name, number) VALUES ('David Driver', '319700002') ON CONFLICT (number) DO UPDATE SET name=excluded.name",
		"INSERT INTO drivers (name, number) VALUES ('Eileen LaRue', '319700003') ON CONFLICT (number) DO UPDATE SET name=excluded.name",
		"INSERT INTO proxy_numbers (number) VALUES ('319700004') ON CONFLICT (number) DO NOTHING",
		"INSERT INTO proxy_numbers (number) VALUES ('319700005') ON CONFLICT (number) DO NOTHING",
	}
	dbInsert(insertData)
}

// Person is a person
type Person struct {
	ID     int
	Name   string
	Number string
}

// ProxyNumberType templates proxy numbers
type ProxyNumberType struct {
	ID     int
	Number string
}

// RideType templates rides
type RideType struct {
	ID              int
	Start           string
	Destination     string
	DateTime        string
	ThisCustomer    Person          // foreign key
	ThisDriver      Person          // foreign key
	ThisProxyNumber ProxyNumberType // foreign key
	NumGrp          [][]int         // Number groups for proxy number rotation
}

// RideSharingDB outlines overall rideshare data structure
type RideSharingDB struct {
	Customers    map[int]Person
	Drivers      map[int]Person
	ProxyNumbers map[int]ProxyNumberType
	Rides        map[int]RideType
	Message      string // For misc messages to be displayed in rendered page
}

func (dbdata *RideSharingDB) loadDB() error {
	db, err := sql.Open("sqlite3", "./ridesharing.db")
	if err != nil {
		return err
	}
	defer db.Close()

	hereCustomers := make(map[int]Person)
	hereDrivers := make(map[int]Person)
	hereProxyNumbers := make(map[int]ProxyNumberType)
	hereRides := make(map[int]RideType)

	q := "SELECT * FROM customers"
	rows, err := db.Query(q)
	if err != nil {
		return err
	}
	for rows.Next() {
		var thisPerson Person
		err := rows.Scan(&thisPerson.ID, &thisPerson.Name, &thisPerson.Number)
		if err != nil {
			log.Println(err)
		}
		hereCustomers[thisPerson.ID] = thisPerson
	}

	q2 := "SELECT * FROM drivers"
	rows2, err := db.Query(q2)
	if err != nil {
		return err
	}
	for rows2.Next() {
		var thisPerson Person
		err := rows2.Scan(&thisPerson.ID, &thisPerson.Name, &thisPerson.Number)
		if err != nil {
			log.Println(err)
		}
		hereDrivers[thisPerson.ID] = thisPerson
	}

	q3 := "SELECT * FROM proxy_numbers"
	rows3, err := db.Query(q3)
	if err != nil {
		return err
	}
	for rows3.Next() {
		var thisNumber ProxyNumberType
		err := rows3.Scan(&thisNumber.ID, &thisNumber.Number)
		if err != nil {
			log.Println(err)
		}
		hereProxyNumbers[thisNumber.ID] = thisNumber
	}

	q4 := "SELECT * FROM rides"
	rows4, err := db.Query(q4)
	if err != nil {
		return err
	}
	for rows4.Next() {
		var thisRide RideType
		err := rows4.Scan(&thisRide.ID, &thisRide.Start, &thisRide.Destination, &thisRide.DateTime, &thisRide.ThisCustomer.ID, &thisRide.ThisDriver.ID, &thisRide.ThisProxyNumber.ID)
		if err != nil {
			log.Println(err)
		}

		// Because the structure of our RideType struct uses
		// nested structs to represent the customer, driver, and proxy number
		// instead of relying on an SQL join to get data for the foreign keys
		// in our 'rides' table, we're looping over data we've already gotten from
		// our earlier SELECT queries and assigning them directly to the fields of
		// the current RideType struct in our map.
		// NOTE: This only works because we don't intend to write to our struct
		// any persistent changes. Any changes to our data has to be written directly to
		// our database, and not to our structs which are meant only for displaying data
		// on rendered views.
		for k1, v1 := range hereCustomers {
			if k1 == thisRide.ThisCustomer.ID {
				thisRide.ThisCustomer.Name = v1.Name
				thisRide.ThisCustomer.Number = v1.Number
			}
		}
		for k2, v2 := range hereDrivers {
			if k2 == thisRide.ThisDriver.ID {
				thisRide.ThisDriver.Name = v2.Name
				thisRide.ThisDriver.Number = v2.Number
			}
		}
		for k3, v3 := range hereProxyNumbers {
			if k3 == thisRide.ThisProxyNumber.ID {
				thisRide.ThisProxyNumber.Number = v3.Number
			}
		}
		thisRide.NumGrp = append(thisRide.NumGrp, []int{thisRide.ThisCustomer.ID, thisRide.ThisProxyNumber.ID})
		thisRide.NumGrp = append(thisRide.NumGrp, []int{thisRide.ThisDriver.ID, thisRide.ThisProxyNumber.ID})
		hereRides[thisRide.ID] = thisRide
	}
	*dbdata = RideSharingDB{hereCustomers, hereDrivers, hereProxyNumbers, hereRides, ""}
	return nil
}
