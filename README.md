# Masked Phone Numbers
### ⏱ 45 min build time

## Why Build a Number Masking Application?

Online service platforms, such as ridesharing, online food delivery and
logistics, facilitate the experience between customers and providers by
matching both sides of the transaction to ensure everything runs smoothly and
the transaction is completed. Both get what they need, and everyone's happy :)

Sometimes, though, the experience doesn't quite go to plan and it becomes
necessary for customers and providers to talk to or message each other
directly. We don't want to reveal their phone numbers to each other either —
that would be a breach of privacy. Instead, we need a way to mask their phone
numbers while allowing them to contact each other directly.

MessageBird allows you to do this with proxy phone numbers that can mask a
user's personal phone number while also protecting the provider's personal
contact details. As a result, neither the customer nor the provider sees the
other party's phone number. Instead, both are directed to call or send messages
to a proxy phone number, which then relays the call or message to the party
meant to receive it.

In this MessageBird Developer Guide, we'll show you how to build a proxy system in Go to mask phone
numbers for our fictitious ridesharing platform, BirdCar.

## Getting Started

Before we get started building our Go application, make sure that you've
installed the following:

- Go 1.11 and newer.
- [MessageBird Go SDK](https://github.com/messagebird/go-rest-api)
5.0.0 and newer.

Install the MessageBird Go SDK with the `go get` command:

```bash
go get -u -v github.com/messagebird/go-rest-api
```

To keep the guide straightforward, we'll be using the Go standard library for
most of our application, and a SQLite3 database to store the data model that
powers our ridesharing application. You may want to use a different RDBMS for a
production-ready implementation; the SQL statements used in this guide should be
transferable to any RDBMS that uses SQL.

To work with and connect to a SQLite3 database, we'll need to install
[mattn](https://www.github.com/mattn)\'s SQLite3 driver for Go,
[`go-sqlite3`](https://github.com/mattn/go-sqlite3):

```bash
go get -u -v github.com/mattn/go-sqlite3
```

**NOTE**: You need to have `gcc` installed in order to build your application
with `go-sqlite3`. See the [`go-sqlite3`
documentation](https://github.com/mattn/go-sqlite3) for more information.

Once you've done all that, we can move on to
[structuring our application](#structuring-our-application).

## Structuring Our Application

Our BirdCar ridesharing service seeks to pair customers who need a car ride with
drivers looking for passengers. When a customer contacts BirdCar to request for
a ride, our application should:

1. Ask the customer for details of the ride they need.
2. Pair the customer up with a driver.
3. Check our database for a VMN that we can assign to the ride as a proxy number.
4. Once we find an available VMN to use as a proxy number, we send an SMS
notification to both the customer and driver from that proxy number to let them
know that they can use this number to contact the other party for this ride.
5. We then write the ride's details to our database.
6. When one party calls or sends an SMS message to a proxy number, our
application relays that call or SMS message to the other party for that ride.

To get our application to do all of the above, we need to build the following:

* [**Data model**](#building-our-data-model): We need to build a data model to
store ride information.
* [**MessageBird Flows**](#messagebird-flows):
    * A MessageBird flow that forwards SMSes received
		by our VMNs to a webhook URL.
    * A MessageBird flow that forwards calls received
		by our VMNs to another webhook URL.
* [**Web Application**](#web-application): Our web application should:
    * Read from and write to our database.
    * Publish an administrator's interface to manage BirdCar rides.
    * Handle POST and GET requests from MessageBird flows.
    * When a ride is added, send an SMS notification
		to the customer and driver for that ride.
    * When a customer or driver sends an SMS message
		to a VMN, our application should detect a POST request
		at a given URL and relay the message to its intended recipient.
    * When a customer or driver calls a VMN, our application should detect a
		GET request at a girl URL and relay the call to its intended recipient.

With this in mind, we can start building your application. We'll write our Go
code in four separate files so that it's easier to read and understand:

- `main.go` contains our application's `main()` block.
- `db.go` contains the code that interacts with our SQLite3 database.
- `routes.go` contains code that defines our HTTP routes.
- `routeHelpers.go` contains code that defines helper functions we'll use when
writing our HTTP routes.

These files should be located at your project root. To run your application, run
the following command in the terminal:

```bash
go run *.go
```

First, we'll initialize and configure the MessageBird Go SDK in `main.go`.

## Configure the MessageBird Go SDK

We'll need to configure the MessageBird Go SDK with a valid API key in order to
make calls and send SMS messages with the MessageBird REST API.

First, create a file named `.env` in your project directory. Then, get your
MessageBird API key from the
[API access (REST) tab](https://dashboard.messagebird.com/en/developers/access)
in the _Developers_ section of your MessageBird account, and write it into your
`.env` file like so:

```
MESSAGEBIRD_API_KEY=<enter-your-api-key-here>
```

Then, run the following commands in your terminal to load your API key as the
`MESSAGEBIRD_API_KEY` environment variable:

```bash
source .env
export MESSAGEBIRD_API_KEY
```

Once that's done, add the following code to `main.go` to initialize the
MessageBird Go SDK:

```go
// main.go
package main

import (
  "log"

  messagebird "github.com/messagebird/go-rest-api"
)

func main(){
  mb := messagebird.New(os.Getenv("MESSAGEBIRD_API_KEY"))

}
```

You can also use a library like [GoDotEnv](https://github.com/joho/godotenv) to
automatically load environment variables from a configuration file.

## Building Our Data Model

Because Go is strict about handling data types, we'll be building our data model
first to help us understand how to build the rest of our application around our
data.

Judging from our [application structure](#structuring-our-application), we know that
our database should contain the following tables:

- Customer data
- Driver data
- Proxy number pool
- Ride information

In the following sections, we'll write the SQL statements to
[initialize our ridesharing database](#initialize-ridesharing-database) and
then figure out how to read data from it into
[data structures](#set-up-data-structures) we set up in our application.

### Initialize Ridesharing Database

First, let's initialize our ridesharing database.
Add the following code to our `db.go` file:

```go
// db.go
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
}
```

In the above code snippet, we've:

* Defined two helper functions: `must()`, which logs any errors encounters and
exits the program, and `dbInsert()`, which prepares and executes a list of SQL
statements passed into it.
* We then write a series of SQL statements to execute, and pass that into a
`dbInsert()` call.
* In our SQL statements, we've:
	* Created four tables: `customers`, `drivers`, `proxy_numbers`, and `rides`
	* Made sure that each SQL `CREATE TABLE` statement is idempotent by writing
	`IF NOT EXISTS`, so we don't attempt to insert tables that already exist into
	our database. This means that `initExampleDB()` can be safely run multiple
	times, even if we've already initialized the database.
* We've set phone numbers (`numbers`) as `UNIQUE` to make sure that we don't
get duplicate phone numbers. This is
important because we will be using phone numbers to identify who to relay SMS
messages and phone calls to.

Next, we'll add example data into our database. When writing your
production-ready application, remember to replace these with actual data. Add
the following lines of code to the bottom of your `initExampleDB()` block:

```go
// db.go
func initExampleDB(){
	// ...
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
```

Here, we've added `ON CONFLICT (number) ...` to each SQL statement because
unlike names, we need phone numbers in our database to be unique (which we've
covered above). You may want to replace the phone numbers in the examples above
with working phone numbers to allow you to test your application. But for now,
we're just concerned with getting the shape of our data right, so we can begin
writing code to read data from our database into our ridesharing application.

### Set Up Data Structures

Now that we've written the code to initialize our database, we can start writing
code to read data out from it into our application. The `sql` package from the
Go standard library allows us to run SQL queries on a database by calling
`db.Query("SELECT * FROM your_table")`, which then gives you a `Rows` struct
that you have to unpack.

In this section, we'll cover the following topics:

- [How to Read from a Database with Go](#how-to-read-from-a-database-with-go)
- [Defining the Structs that Contain Data](#defining-the-structs-that-contain-data)
- [Load Data into Data Structures](#load-data-into-data-structures)

#### How to Read from a Database with Go

Because Go is a strictly typed language, the code to read data from databases is
slightly more verbose. For example, if we run a `SELECT` query to read from our
`customers` table, we have to unpack the `Rows` struct we receive by writing the
following code:

```go
// Example

// These variables can have any name, but must be the same type as the data we're going to copy into it.
var (
	customerID int
	customerName string // 'text' type in the database
	customerNumber string // 'text' type in the database
)
rows, _ := db.Query("SELECT * FROM customers")
for rows.Next() {
	rows.Scan(&customerID,&customerName,&customerNumber)
	log.Printf("ID: %d\nName: %s\nPhone Number: %s\n", customerID, customerName, customerNumber)
}
```

The code snippet above does the following:

1. It runs a database query (we're discarding the error to keep our example
brief) and saves the `*sql.Rows` struct that's returned to the `rows` variable.
2. It then iterates through the records stored in `rows` with a
`for rows.Next()` loop. When `rows.Next()` returns false, it means we've run
out of records to process and can exit the loop.
4. For each record we find in `rows`, we call `rows.Scan()` to scan the columns
of that record.
5. For each column that the record contains, we pass in the address of the
variable we want to copy the value contained in that column for that record.
For example, for a record that has the columns "id" and "animal", we copy the
values contained in these columns to variables we've already defined by calling
`rows.Scan(&idVariable, &animalVariable)`.
6. The variables whose addresses we pass into our `rows.Scan()` call must
fulfill the following:
	* Each of the variables whose address we pass into the `rows.Scan()` call
	must be of the correct type for that corresponding column. For example, if
	we're trying to copy out a `text` value from the record, we must pass in a
	variable of type `string`.
	* `rows.Scan()` must have exactly the same number of addresses passed into it
	as the number of columns in the record.
		* If `rows.Scan()` contains a different number of addresses than the number
		of columns the record contains (more or fewer than expected), it returns an
		error and no values are copied out — causing your program to seem to have
		read an empty record.
		* For example, for a record with the columns "id" and "animal", attempting to call
		`rows.Scan(&firstVar, &secondVar, &thirdVar)` will not copy any values to
		`firstVar`, `secondVar`, or `thirdVar` — instead, it returns the error:
		`sql: expected 2 destination arguments in Scan, not 3`.

#### Defining the Structs that Contain Data

We need to read from the four tables we've created in our database —
`customers`, `drivers`, `proxy_numbers`, `rides` — and store them in some kind
of data structure within our Go application. But before attempting to read data
out from the database into our application, we have to define the shape of the
data we expect to get from the database.

**NOTE**: In your production application, you may want to implement a form of
paging where you wouldn't read and copy all the data from your database into
your application at one go. But to keep this guide straightforward, we'll have
our application load the entire database into one struct that we pass to our
application when rendering views.

We'll do this by describing the shape of the structs we'll be using to store
data from these tables as struct `type`s. At the bottom of your `db.go` file,
add the following lines of code:

```go
// db.go
// Person is a person, to whom we assign a ID, Name, and Number.
// Used to represent Customers and Drivers
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
```

In the above code example:

* We are defining several struct types to contain data we expect to get from
the database.
* `type Person struct` describes a "person" as stored in our
`customers` and `drivers` table.
* `type ProxyNumberType` describes a "Proxy Number" or a
"VMN" as stored in our `proxy_number` table.
* `type RideType struct` describes a single "Ride" as stored
in our `rides` table.
	* Notice that we're "inheriting" types in our `RideType` struct for the
	`ThisCustomer`, `ThisDriver`, and `ThisProxyNumber` fields. This allows us to
	nest data for each ride, instead of relying on `JOIN` statements to get
	information about the foreign keys that these columns refer to in the `rides`
	table. We can do this because we don't intend to write persistent changes to
	`RideType` — all persistent changes to our data is written directly to our
	database.
	* We also have an additional field named `NumGrp`. This field is used for our
	[proxy number rotation](#proxy-number-rotation) implementation that we will
	write later.
* `type RideSharingDB struct` describes a struct that is an aggregate of all
the data that we need to pass to our rendered views.
	* In it, notice that we've shadowed our tables with `map` types. We'll get
	into how this works when we write the code for populating these data
	structures with data from the database.
	* We also define a `Message` type in this struct, which we will use to pass error messages or similar to be displayed in our rendered views.

### Load Data into Data Structures

Once we've defined our data structures, we need to write a helper method that
loads data into any RideSharingDB struct that we define and return it for the rest of our application to use.

Add to the bottom of your `db.go` file the following lines of code:

**NOTE**: For brevity, we're not including the full code snippet. For the complete example application, go to the [MessageBird Developer Guides GitHub repository](https://github.com/messagebirdguides/masked-numbers-guide-go).

```go
// db.go
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

 // ...
 // We're only including part of the code necessary for your application
 // to work. For the full code example, go to:
 // https://github.com/messagebirdguides
 // ...

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
```

In the above code sample:

1. We're writing a method, with our RideSharingDB struct type as the method
receiver. This allows us to load data into any `RideSharingDB` struct with this
method.
2. We're loading the ridesharing database into the variable `db`.
3. Then we initialize the maps that will contain data that we read from our
tables by using `make()`.
4. Once we've done that, we write our queries.
5. For each query, we define a query statement `q` which we pass into
`db.Query()` and get a `rows` struct.
6. For each `rows` struct we get, we write a `for rows.Next()` loop that loops
through each record in the table we've read.
7. For each record we read, we define a container variable (e.g. thisPerson)
that we copy values from the record into, and then append that variable to the
corresponding map we've initialized in step 3, using the `ID` of that record as
the map key.
8. Once we've done this for all four tables, we rewrite the RideSharingDB
struct that is attached to this method with the data we've read off the
database with the following line of code:
`*dbdata = RideSharingDB{hereCustomers, hereDrivers, hereProxyNumbers, hereRides, ""}`

When this helper method is called, it loads data from the database into the struct it is attached to.

For example, if we define a struct with `thisDatabase := new(RideSharingDB)`,
and then call `thisDatabase.loadDB()`, it reads data from the database and loads
it into the corresponding fields — `thisDatabase.Customers`,
`thisDatabase.Drivers`, `thisDatabase.ProxyNumbers`, `thisDatabase.Rides` — to
populate it.

We then can pass `thisDatabase` into any template to display the data in a rendered view.

## MessageBird Flows

Now that we've got all our data structures set up, we can move on to configuring
our MessageBird account to receive calls and SMS messages, and then forwarding
them to their intended recipients.

To do this, we're going to:

1. [Expose our local development server with localtunnel.me](#expose-local-development-server)
2. [Prepare one or more VMNs](#prepare-vmns)
3. For each VMN you will be using in your [number pool](#proxy-number-rotation), you have to:
	* [Connect the VMN to a Webhook for SMS](#connect-the-vmn-to-a-webhook-for-sms)
	* [Connect the VMN to a Webhook for Voice](#connect-the-vmn-to-a-webhook-for-voice)

### Expose Local Development Server

We need to expose our development environment to the MessageBird servers in
order for the [MessageBird flows](#messagebird-flows) to work. You can use tools
such as [localtunnel.me](https://localtunnel.me) or [ngrok](https://ngrok.com/)
that provides a public URL to connect to a locally running server.

You can install [localtunnel.me](https://localtunnel.me) with npm:

```bash
npm install -g localtunnel
```

To expose a server running on port 8080, run:

```bash
lt --port 8080
```

The terminal then displays the URL at which you can access your application:

```bash
your url is: https://<assigned_subdomain>.localtunnel.me
```

**NOTE**: Whenever you run the `lt` command, localtunnel.me starts a new
`lt` instance that has a different unique URL assigned. Because you have to
assign a static URL for MessageBird to make webhook requests, quitting and
running `lt` again will change the URL for your local development server,
causing MessageBird to be unable to contact it until you update your flows with
your new URL.

### Prepare VMNs

In order to receive messages, you need to have set up one or more Virtual Mobile
Number (VMN) in your MessageBird account. VMNs look and work just like regular
mobile numbers. However, they live in the cloud instead of being attached to a
mobile device via a SIM card i.e., a data center, and can process incoming SMS
and voice calls. Explore our low-cost programmable and configurable numbers
[here](https://www.messagebird.com/en/numbers).

Here's how to purchase one:

1. Go to the [Numbers](https://dashboard.messagebird.com/en/numbers)
section of your MessageBird account and click **Buy a number**.
2. Choose the country in which you and your customers are located
and make sure both the _SMS_ and _Voice_ capabilities are selected.
3. Choose one number from the selection and the duration for which
you want to pay now. ![Buy a number screenshot](/assets/images/screenshots/maskednumbers/buy-a-number.png)
4. Confirm by clicking **Buy Number**.

Congratulations, you have set up your first VMN!

One VMN is enough for testing your application, but you'll need a larger pool of
numbers for a production-ready implementation of the ridesharing service. Follow
the same steps listed above to purchase more VMNs.

### Connect the VMN to a Webhook for SMS

You've got a VMN now, but MessageBird has no idea what to do with it. To start
using your VMN with your applications you need to define a _Flow_ next that ties
your number to a webhook that tells MessageBird what it should do with the VMN.
We'll start with the flow for incoming SMS messages:

1. Go to the [Flow Builder](https://dashboard.messagebird.com/en/flow-builder)
section of your MessageBird account. Under "Navigate to", select **Templates**.
In the list of templates there, find the one named "Call HTTP endpoint with SMS"
and click "Try this flow".
![Create Flow, Step 1](/assets/images/screenshots/maskednumbers-go/create-sms-flow-1.jpg)
2. Give your flow a name, such as "Number Proxy for SMS".
3. The flow template should contain two steps.
Click on the first step, "SMS".
In the dialog box that pops up,
Select all the VMNs that this flow should apply to.
![Create Flow, Step 2](/assets/images/screenshots/maskednumbers-go/create-sms-flow-2.jpg)
4. Click on the second step, "Forward to URL". In the dialog box that appears,
select _POST_ in the **Method** field, and copy your
[localtunnel.me URL](#expose-local-development-server) into the **URL** field.
Add `/webhook` to the end of your **URL** - this is the name of the route
we will use to handle incoming messages. Click **Save**.
![Create Flow, Step 3](/assets/images/screenshots/maskednumbers-go/create-sms-flow-3.jpg)
5. Click **Publish Changes** to activate your flow.

### Connect the VMN to a Webhook for Voice

Set up a second flow to configure your VMNs to process incoming voice calls:

1. Go back to the [Flow Builder](https://dashboard.messagebird.com/en/flow-builder)
and select "New Flow", and then click on "Create Custom Flow".
2. In the **Set up new flow** dialog box that displays, enter a **Flow Name**
and select "Phone Call" as the **Trigger**. Click **Next**.
![Create Voice Flow, Step 1](/assets/images/screenshots/maskednumbers-go/create-voice-flow-1.jpg)
3. This should take you to a new Flow Builder page with a "Phone Call" step; click on it.
In the dialog box that appears to the right,
select all the VMNs you want this flow to apply to, and click **Save**.
![Create Voice Flow, Step 2](/assets/images/screenshots/maskednumbers-go/create-voice-flow-2.jpg)
4. From the **Steps** panel on the left, drag the "Fetch Call Flow from URL"
step to the spot just under your "Phone Call" step.
![Create Voice Flow, Step 3](/assets/images/screenshots/maskednumbers-go/create-voice-flow-3.jpg)
5. Select the "Fetch Call Flow from URL" step, and add your [localtunnel.me URL](#expose-local-development-server) into the **Call flow URL** field.
Add `/webhook-voice` to the end of your **Call flow URL**, and click **Save**.
![Create Voice Flow, Step 4](/assets/images/screenshots/maskednumbers-go/create-voice-flow-4.jpg)
6. Click **Publish Changes** to activate your flow.

You're done setting up flows for your application! Now, we can begin writing
routes in your application for the `/webhook` and `/webhook-voice` URL paths
that these flows are using.

## Web Application

Now we can start writing the web server component of your application. We won't
go through how to write Go HTML templates or the basics of HTTP routing —
instead, we'll be focusing on routing logic.

First, let's review what we need our web server to do:

* Provide a administrative interface for managing BirdCar rides.
On this administrative interface, we should be able to:
	* Browse all known customers, drivers, and proxy numbers.
	* Create new rides.
* Listen on our webhook URLs for the following:
	* Listens for POST requests on `/webhook` to handle any SMS messages
	forwarded to the web server from our VMNs by the MessageBird API.
	* Listens for GET requests on `/webhook-voice` to handle any voice calls
	forwarded to the web server from our VMNs by the MessageBird API.

Our web server code can be found in
the following locations in the sample code repository:

- `views/`: This contains all our Go HTML templates. `default.gohtml` contains
the code for our base layout, while `landing.gohtml` contains the code for our landing
page template. When rendered, `landing.gohtml` uses the Go HTML templating syntax to pull
data from the struct (of type `RideSharingDB`) that we pass into when when executing the
template.
- `routes.go`: Contains our route handlers. Here, we'll be writing code that handles
the POST and GET requests that our web server receives, as well as send
MessageBird SMS messages and make voice calls when needed.
- `routeHelpers.go`: Contains code for helpers that we use in `routes.go`.
- `main.go`: We'll need to add code here that initializes our database, defines our
routes, and starts the web server.

In this section, we'll cover the following topics:

- [Stubbing Out Routes](#stubbing-out-routes)
- [Writing a Template Rendering Helper](#writing-a-template-rendering-helper)
- [Building an Admin Interface](#building-an-admin-interface)
- [Writing a Proxy Number Availability Helper](#writing-a-proxy-number-availability-helper)
- [Writing a Helper to Send SMS Messages](#writing-a-helper-to-send-sms-messages)
- [Writing our Message Webhook Handler](#writing-our-message-webhook-handler)
- [Writing our Voice Call Webhook Handler](#writing-our-voice-call-webhook-handler)

### Stubbing Out Routes

First, we'll stub out our routes in `main.go`. Rewrite your `main()` block in
`main.go` to look like the following:

```go
// main.go

// Remember to add the `net/http` package to your import statement.
func main() {
	dbdata := new(RideSharingDB)
	initExampleDB()

	mb := messagebird.New(os.Getenv("MESSAGEBIRD_API_KEY"))

	mux := http.NewServeMux()
	mux.Handle("/", landing(dbdata))
	mux.Handle("/createride", createRideHandler(dbdata, mb))
	mux.Handle("/webhook", messageHookHandler(dbdata, mb))
	mux.Handle("/webhook-voice", voiceHookHandler(dbdata, mb))

	port := ":8080"
	log.Println("Serving on", port)
	err := http.ListenAndServe(port, mux)
	if err != nil {
		log.Fatal(err)
	}
}
```

Here, we've:

* Initialized a `dbdata` struct that uses our `RideSharingDB` type.
We'll be passing this to our handlers to update and display in rendered views.
* We've also initialized our MessageBird Go client, and saved it as `mb`.
We'll also pass this to our handlers to make requests to the MessageBird API.
* Then, we stub out four routes:
	* `/`: This is our default route, and will be handled by `landing()`.
	* `/createride`: This is the route for creating new rides,
	and will be handled by `createRideHandler()`.
	* `/webhook`: This is the route on which we'll be listening for
	POST requests from the MessageBird server when one of our VMNs
	receives an SMS message, and will be handled by `messageHookHandler()`.
	* `/webhook-voice`: This is the route on which we'll be listening for
	GET requests from the MessageBird server when one of our VMNs
	receives a call, and will be handled by `voiceHookHandler()`.
* With all that done, we then initialize our web server with by calling
`http.ListenAndServe()`.

### Writing a Template Rendering Helper

The code that loads and executes our templates to render a view can be offloaded
to a helper, that we'll write in our `routeHelpers.go` file.

In there, we've written our `renderDefaultTemplate()` helper:

```go
// routeHelpers.go

// ...

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
```

Using this, we can render a view in a `http.HandlerFunc()` by writing `renderDefaultTemplate(w, <template-file-to-render>, <data-to-display>)`.
For example, for our `landing()` handler, we write:

```go
func landing(dbdata *RideSharingDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// ...
		renderDefaultTemplate(w, "views/landing.gohtml", dbdata)
	}
}
```

Writing our `renderDefaultTemplate()` helper this way also means that we can
use the same line of code to update the page whenever whenever our ridesharing
database is updated:

```go
// Example
func updateExample(dbdata *RideSharingDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		db, _ := dbdata.loadDB()
		if r.Method == "POST" {
			// Get a message from POST data.
			r.ParseForm()
			// Copies message to our dbdata struct Message field.
			message := r.FormValue("text_from_POST_submission")
			dbdata.Message = message
			// Render view, with updated dbdata struct.
			renderDefaultTemplate(w, "views/landing.gohtml", dbdata)
			// Must return, or handler will instruct our application to
			// continue running subsequent code.
			return
		}
		renderDefaultTemplate(w, "views/landing.gohtml", dbdata)
	}
}
```

### Building an Admin Interface

Next, we'll create the administrator's interface for our web application.
To keep things brief, we won't cover the sample code in that much detail.
For the full code sample, go to the [MessageBird Developer Guides GitHub repository](https://github.com/messagebirdguides/masked-numbers-guide-go).

Our `landing.gohtml` needs to render the following
fields from our `dbdata *RideSharingDB` struct for our admin interface:

- `dbdata.Customers`: Our list of known customers, which we display as a dropdown menu from which an administrator can select a customer when creating a new ride.
- `dbdata.Drivers`: Our list of known drivers, which we display as a dropdown menu from which an administrator can select a drivers when creating a new ride.
- `dbdata.ProxyNumbers`: Our list of VMNs in our proxy number pool,
displayed as a table. Our ridesharing service should randomly
assign an available proxy number when a new ride is created.
- `dbdata.Rides`: Our list of rides, displayed as a table.
- `dbdata.Messages`: This should contain any messages, usually error messages,
that we want to display on our rendered view. By default, this should be set to
an empty string value (`""`).

Our `"/"` route, which renders our admin interface, should do only two things:

1. Load our ridesharing database.
2. Execute and render our templates, having passed in data we've loaded
from our database. Our `renderDefaultTemplate()` helper
[helps us with this](#write-template-rendering-helper).

Other route handlers that display a page follow a similar execution path.
For example, our `/createride` route does the following:

1. Load our ridesharing database.
2. Collects data submitted through a POST request.
3. Updates our ridesharing database.
4. Re-loads our ridesharing database.
5. Notifies the customer and driver that they've been assigned a new ride and VMN.
6. Executes and renders our templates, having passed in updated data
we've loaded from our database.

In the above process, only steps 2, 3, and 5 require new code.

For more details on how we do this for the rest of the routes and handlers,
see the sample code in the [MessageBird Developer Guides GitHub repository](https://github.com/messagebirdguides/masked-numbers-guide-go).

### Writing a Proxy Number Availability Helper

In our `routes.go` file, the handler for our `/createride` route creates a
new ride by reading submitted form data that contains a customer ID and driver ID,
and from that compiles the information we need for the new ride. Part of the
information set that we need to create a ride is the VMN we should assign to
the ride.

The VMN assigned to the ride acts as a proxy number, which the customer or
driver for that ride can call to contact the other party
instead of contacting that party directly. We need to write a function
that decides which VMN is available for assignment, and returns it for
use by our application.

In some proxy number systems, a unique VMN is assigned per user, or per
transaction. We want to be a bit more economical than that, and instead use a
proxy number system that assigns a unique VMN per set of customers and drivers.

To illustrate, let's say we have customers A and B, and drivers C and D:

* A ride assigned to customer A and driver C will use VMN_1.
* A ride assigned to customer A and driver D will use VMN_2.
* A ride assigned to customer B and driver C will use VMN_3.
* But for a ride assigned to customer B and driver D,
we can reuse VMN_1 because it has not been previously associated
with either customer B or driver D.

By using this system, we can:

* Rotate VMNs, instead of having to keep a large pool of VMNs for unique assignments.
* Identify rides with a combination of a customer's phone number
and the VMN used, or a driver's phone number and the VMN used.
We'll use this to our advantage when writing our `/webhook` and `/webhook-voice`
route handlers.

To write our helper function, we'll start out with defining our function's inputs.
We know that we'll use this function in a handler, where we'll be getting the ID
of one customer and one driver, so we'll write our helper function to take our
database struct (`dbdata *RideSharingDB`), a customer ID (`customerID int`),
and a driver ID (`driverID int`), and returns a proxy number (of `ProxyNumberType` type) or an error. Add the following code to the bottom of your `routeHelpers.go` file:

```go
// routeHelpers.go
// ...
func getAvailableProxyNumber(dbdata *RideSharingDB, customerID int, driverID int) (ProxyNumberType, error) {
	return ProxyNumberType{}, nil
}
```

We also know that we can assign any VMN to the next ride if it is the first
ride in the database. Modify `getAvailableProxyNumber()` to look like the following:

```go
// routeHelpers.go
// ...
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
	// If we're here, then we've failed to get a proxy number; return error
	return (ProxyNumberType{}), fmt.Errorf("no available proxy numbers")
}
```

Next, we know that we want to identify rides by a combination of
the customer's phone number, driver's phone number,
and the VMN for that ride.
Remember that extra struct field that we
[defined in our `RideType` struct](#defining-the-structs-that-contain-data)
, `NumGrp`? If we go back to the
[Load Data into Data Structures](#load-data-into-data-structures)
section, we'll see close to the bottom of our `loadDB()` block that we have
these lines of code:

```go
// db.go
func (dbdata *RideSharingDB) loadDB() error {
	// ...
	thisRide.NumGrp = append(thisRide.NumGrp, []int{thisRide.ThisCustomer.ID, thisRide.ThisProxyNumber.ID})
	thisRide.NumGrp = append(thisRide.NumGrp, []int{thisRide.ThisDriver.ID, thisRide.ThisProxyNumber.ID})
	// ...
}
```

This means that for every ride, we're populating its `NumGrp` field with a list
of `[]int`s that tells us which combinations of customer IDs, driver IDs, and
proxy number IDs that ride contains. This allows us to quickly compare
check if a ride contains a given combination with the following `containsNumGrp()`
function:

```go
// routeHelpers.go
func getAvailableProxyNumber(/*...*/) (/*...*/){
	// ...
	// Checks if []int contains an int
	containsNumGrp := func(arr [][]int, findme []int) bool {
		for _, v := range arr {
			if reflect.DeepEqual(v, findme) {
				return true
			}
		}
		return false
	}
	// ...
}
// ...
```

Next, we create a flat list of the contents of all the `NumGrp` fields in our
database, so that its easily accessible via a `rideProxySets` variable:

```go
// routeHelpers.go
func getAvailableProxyNumber(/*...*/) (/*...*/){
	// ...
	var rideProxySets [][]int
	for _, v1 := range dbdata.Rides {
		for _, v := range v1.NumGrp {
			rideProxySets = append(rideProxySets, v)
		}
	}
	// ...
}
```

Once that is done, we're finally ready to perform the actual check.
At the bottom of our `getAvailableProxyNumber()` block,
add the following lines of code just before the final return statement:

```go
// routeHelpers.go
func getAvailableProxyNumber(/*...*/) (/*...*/){
	//...
	for _, v2 := range dbdata.ProxyNumbers {
		// Check if both customer/driver+proxy number sets do not exist in current proxy sets
		if !containsNumGrp(rideProxySets, []int{customerID, v2.ID}) && !containsNumGrp(rideProxySets, []int{driverID, v2.ID}) {
			return v2, nil
		}
	}
	// If we end up here, then we've failed to get a proxy number
	return (ProxyNumberType{}), fmt.Errorf("no available proxy numbers")
}
// ...
```

The final block of code should look like this:

```go
// routeHelpers.go
func getAvailableProxyNumber(dbdata *RideSharingDB, customerID int, driverID int) (ProxyNumberType, error) {
	if len(dbdata.Rides) == 0 {
		for _, v := range dbdata.ProxyNumbers {
			return v, nil
		}
		return (ProxyNumberType{}), fmt.Errorf("no available proxy numbers")
	}

	containsNumGrp := func(arr [][]int, findme []int) bool {
		for _, v := range arr {
			if reflect.DeepEqual(v, findme) {
				return true
			}
		}
		return false
	}

	var rideProxySets [][]int
	for _, v1 := range dbdata.Rides {
		for _, v := range v1.NumGrp {
			rideProxySets = append(rideProxySets, v)
		}
	}

	for _, v2 := range dbdata.ProxyNumbers {
		if !containsNumGrp(rideProxySets, []int{customerID, v2.ID}) && !containsNumGrp(rideProxySets, []int{driverID, v2.ID}) {
			return v2, nil
		}
	}

	return (ProxyNumberType{}), fmt.Errorf("no available proxy numbers")
}
```

In our `routes.go` file, we call the `getAvailableProxyNumber()` helper like this:

```go
// routes.go
// ...
availableProxy, err := getAvailableProxyNumber(dbdata, customerIDint, driverIDint)
if err != nil {
	dbdata.Message = fmt.Sprintf("We encountered an error: %v", err)
	log.Println(err)
	renderDefaultTemplate(w, "views/landing.gohtml", dbdata)
	return
}
// ...
```

### Writing a Helper to Send SMS Messages

We'll also want to write a helper for sending SMS messages using the MessageBird
API, so that we can encapsulate the error handling inside a function call.
At the bottom of `routeHelpers.go`, add the following lines of code:

```go
// routeHelpers.go
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
```

Once you've done this, you can call the `mbSender()` function to send SMS messages to
a destination number, like what we've done with `createRideHandler()` in `routes.go`:

```go
func createRideHandler(/*...*/) http.HandlerFunc {
	// ...
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
	// ...
}
```

### Writing our Message Webhook Handler

Now, we'll write the handler that handles the POST requests we'll be getting
from the MessageBird server when our VMNs receive an SMS message.

Our webhook handler needs to do the following:

1. Load our ridesharing database.
2. Check if we're receiving a POST request.
3. If we're receiving a POST request, parse the form data submitted.
When the MessageBird servers receives and forwards an SMS message to
a defined webhook URL, our web application receives it as a map similar
to the following:

	```go
	map[message_id:[7a76afeaef3743d28d0e2d9362xxxxxx] originator:[1613209xxxx] reference:[4774934xxxx] createdDatetime:[2018-09-24T08:30:59+00:00] id:[f91908b75f9e4b1fba3b96dc4499xxxx] message:[this is a test message] receiver:[1470800xxxx] body:[this is a test message] date:[1537806659] payload:[this is a test message] sender:[1613209xxxx] date_utc:[1537777859] recipient:[1470800xxxx]]
	```

4. We check the parsed form data for an "originator" (sender of the message),
a "receiver" (the VMN that received the message), and aa "payload"
(the body of the sent SMS message).
5. We figure out if the "originator" is a customer or driver. To do this, we'll
add two helper functions to `routeHelpers.go` and call them in our handler:

	```go
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
	```

6. If the "originator" is a customer, then we call `sms.Create()` to send
the "payload" to the driver for that ride. If the "originator" is a driver,
then we send the "payload" to the customer.

The handler you'll end up writing should look like the one below:

```go
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
```

### Writing our Voice Call Webhook Handler

When the MessageBird servers receive a voice call on a VMN we've set up a
[MessageBird voice flow](#connect-the-vmn-to-a-webhook-for-voice) for earlier,
it makes a GET request on the URL we've defined for that flow. When it makes that request, it expects an XML response that defines a [call flow](#https://developers.messagebird.com/docs/voice-calling#call-flows).
That call flow contains instructions for MessageBird to make a voice call.
For more information on how to write XML call flows, see the [MessageBird API Reference](#https://developers.messagebird.com/docs/voice-calling#call-flows)

The handler that we're writing to handle the `/webhook-voice` route needs to
parse that GET request and respond with the correct XML call flow.

Our handler should do the following:

1. Load our ridesharing database.
2. Set our "Content-Type" HTTP header to `application/xml`.
3. Parse the GET request with `r.ParseForm()`.
When the MessageBird servers receives and forwards a voice call to
a defined webhook URL, our web application receives it as a map similar
to the following:

	```go
	map[callID:[2894efe1-63b7-4d37-b006-3aab7fxxxxxx] destination:[1470800xxxx] numberID:[272cca7c-c2d6-4781-9e92-168ba0xxxxxx] source:[1613209xxxx] variables:[{}]]
	```

4. Using the "destination" (the VMN that received the voice call) and the
"source" (the caller), we check if the "source" is a customer or driver.
5. If the "source" is a customer, we respond with an XML call flow that transfers
the call to the driver for that ride, and vice-versa. To do this, we write
a response to the `http.ResponseWriter` with the following lines of code:

	```go
	// where we've saved the number to call as 'forwardToThisNumber'
	fmt.Fprintf(w, "<?xml version='1.0' encoding='UTF-8'?><Transfer destination='%s' make='true' />", forwardToThisNumber)
	return
	```
6. If we cannot find the ride or any target party to tranfer the call to, we
respond with an XML call flow that tells the caller that the call transfer has
failed.

We should end up with a `voiceHookHandler()` that looks like the following:

```go
// routes.go
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
```

You're done!

## Testing Your Application

Check again that:

* You've set up at least one VMN.
* Your VMNs should have two flows — the first waits for SMS messages and forwards
them as a POST request to our application; the second waits for voice calls, and
requests a call flow from our application when it receives one.
* Your localtunnel.me tunnel is still running. Remember that whenever you start a fresh tunnel, you'll get a new URL, so you have to update the flows accordingly. You can also configure a more permanent URL using the `-s` attribute with the `lt` command.

To start your ridesharing application, open a new terminal session and run:

```bash
go run *.go
```

Open your browser and go to http://localhost:8080. Select a customer and a driver
and create a ride. If everything is working, the
phone numbers for the selected customer and driver should receive an SMS
notification.

If you send an SMS message from the customer's phone number to the VMN,
that SMS message should be automatically forwarded to the driver's phone,
and vice-versa. Similarly, using the customer's phone to call the assigned VMN
would automatically forward that call to the driver's phone, and vice-versa.

## Nice work!

You've just built your own number masking system with MessageBird!

You can now use the flow, code snippets and UI examples from this tutorial as an inspiration to build your own application. Don't forget to download the code from the [MessageBird Developer Guides GitHub repository](https://github.com/messagebirdguides/masked-numbers-guide-go).

## Next steps

Want to build something similar but not quite sure how to get started? Please feel free to let us know at support@messagebird.com, we'd love to help!
