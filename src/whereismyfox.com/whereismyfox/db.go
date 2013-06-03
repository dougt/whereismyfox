package main

import (
	"code.google.com/p/gosqlite/sqlite"
	"log"
	"strconv"
	"time"
)

var gConn *sqlite.Conn

func openDb() {
	conn, err := sqlite.Open("db.sqlite")
	if err != nil {
		log.Fatalf("Unable to open the database: %s", err)
	}

	conn.Exec("CREATE TABLE devices(id INTEGER PRIMARY KEY AUTOINCREMENT, date TEXT, email TEXT, deviceName TEXT, pushURL TEXT, lon FLOAT, lat FLOAT, UNIQUE (pushURL));")
	gConn = conn
}

func closeDb() {
	gConn.Close()
	gConn = nil
}

func updateDeviceLocation(pushURL string, lat float64, lon float64) bool {

	err := gConn.Exec("UPDATE devices SET date=" +
		strconv.FormatInt(time.Now().Unix(), 10) +
		", lat=" + strconv.FormatFloat(lon, 'f', 4, 64) +
		", lon=" + strconv.FormatFloat(lat, 'f', 4, 64) +
		" WHERE pushURL='" + pushURL + "'")

	if err != nil {
		log.Println("Error while update location: "+pushURL+" err: ", err)
		return false
	}
	return true
}

func addDevice(email string, deviceName string, pushURL string) bool {

	if email == "" || deviceName == "" || pushURL == "" {
		return false
	}

	log.Println("adding new device: " + deviceName + " to db for user: " + email)
	now := strconv.FormatInt(time.Now().Unix(), 10)

	insertString := "INSERT INTO devices(date, email, deviceName, pushURL) VALUES('" + now + "', '" + email + "', '" + deviceName + "', '" + pushURL + "')"

	err := gConn.Exec(insertString)
	if err != nil {
		log.Fatalf("Error while Inserting: %s", err)
		return false
	}
	return true
}

func deleteDevice(pushURL string) bool {

	delString := "DELETE FROM devices WHERE pushURL='" + pushURL + "'"

	err := gConn.Exec(delString)
	if err != nil {
		log.Fatalf("Error while deleting: %s", err)
		return false
	}
	return true
}

func devicesForUser(email string) []DeviceInformation {

	selectStmt, err := gConn.Prepare("SELECT deviceName, pushURL FROM devices WHERE email='" + email + "';")
	if err != nil {
		log.Fatalf("Error while preparing select: %s", err)
		return nil
	}

	err = selectStmt.Exec()
	if err != nil {
		log.Fatalf("Error while exec select: %s", err)
		return nil
	}

	result := make([]DeviceInformation, 0)

	for selectStmt.Next() {
		var deviceName = ""
		var pushURL = ""
		err = selectStmt.Scan(&deviceName, &pushURL)
		if err != nil {
			log.Fatalf("Error while getting row data: %s", err)
			return nil
		}
		info := DeviceInformation{deviceName, pushURL}
		result = append(result, info)
	}

	return result
}
