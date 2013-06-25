package main

import (
	"encoding/json"
	"fmt"
	"time"
	"log"
	"net/http"
	"strconv"
	"strings"
)

var gDB *DB = nil

type DeviceListResponse struct {
	Devices []Device `json: "devices"`
}

func deviceListHandler(w http.ResponseWriter, r *http.Request) {

	if !IsLoggedIn(r) {
		log.Println("deviceListHandler: user not logged in")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("."))
		return
	}

	loginName := GetLoginName(r)
	if loginName == "" {
		log.Println("deviceListHandler: user does not have an email address")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("."))
		return
	}

	deviceList, err := gDB.ListDevicesForUser(loginName)

	if err != nil || deviceList == nil {
		log.Println("deviceListHandler: device list is empty for user")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("."))
		return
	}

	var data []byte

	response := DeviceListResponse{deviceList}

	data, err = json.Marshal(response)
	if err != nil {
		log.Println("deviceListHandler: could not marshal data")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("."))
		return
	}

	w.Write(data)
}

func deviceAddHandler(w http.ResponseWriter, r *http.Request) {
	if !IsLoggedIn(r) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("."))
		return
	}

	loginName := GetLoginName(r)
	if loginName == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("."))
		return
	}

	err := r.ParseForm()
	if err != nil {
		w.WriteHeader(400)
		w.Write([]byte("Bad Request."))
		return
	}

	pushURL := r.FormValue("pushURL")
	if pushURL == "" {
		w.WriteHeader(400)
		w.Write([]byte("Bad Request. push"))
		return
	}

	deviceName := r.FormValue("deviceName")
	if deviceName == "" {
		w.WriteHeader(400)
		w.Write([]byte("Bad Request."))
		return
	}

	if _, err := gDB.AddDevice(loginName, deviceName, pushURL); err != nil {
		/* FIXME send back device id or url to report coordinates */
		w.Write([]byte("ok"))
		return
	}

	w.WriteHeader(http.StatusInternalServerError)
	w.Write([]byte("."))
}

func deviceLocationHandler(w http.ResponseWriter, r *http.Request) {
	if !IsLoggedIn(r) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("."))
		return
	}

	err := r.ParseForm()
	if err != nil {
		w.WriteHeader(400)
		w.Write([]byte("Bad Request."))
	}

	lat, err1 := strconv.ParseFloat(r.FormValue("lat"), 64)
	lon, err2 := strconv.ParseFloat(r.FormValue("lon"), 64)

	if err1 != nil || err2 != nil {
		w.WriteHeader(400)
		w.Write([]byte("Bad Request."))
		return
	}

	d, err := gDB.GetDeviceById(0) // FIXME some id taken from the url
	if d != nil {
		w.WriteHeader(400)
		w.Write([]byte("Bad Request."))
		return
	}

	if err = gDB.UpdateDeviceLocation(d, lat, lon); err != nil {
		w.Write([]byte("ok"))
		return
	}

	w.WriteHeader(400)
	w.Write([]byte("Bad Request."))
	return
}

func deviceLostHandler(w http.ResponseWriter, r *http.Request) {
	if !IsLoggedIn(r) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("."))
		return
	}

	// check that this device is actually owned by the user
	email := GetLoginName(r)
	if email == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("."))
		return
	}

	lostDevice, err := gDB.GetDeviceById(0) // FIXME this should come from the url
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("."))
		return
	}

	body := fmt.Sprintf("version=%d", uint64(time.Now().Unix()))
	request, err := http.NewRequest("PUT", lostDevice.Endpoint, strings.NewReader(body))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Failed to PUT to endpoint!"))
	}

	request.Header["Content-Type"] = []string{"application/x-www-form-urlencoded"}

	var client http.Client
	_, err = client.Do(request)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Failed to PUT to endpoint"))
	}
}

func serveSingle(pattern string, filename string) {
	http.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		log.Println("serving file " + filename);
		http.ServeFile(w, r, filename)
	})
}

func main() {

	readConfig()
	gDB, err := OpenDB("db.sqlite")

	/*
	GET /device/ -> list of device uris
	PUT {name, endpoint} /device/ -> /device/id
	GET /device/id -> {name, endpoint, latitude, longitude, lostOrFound}
	DELETE /device/id ?

	POST /device/id {latitude, longitude}

	POST lostURL -> mark device as lost
	POST foundURL -> mark device as found
	*/

	http.HandleFunc("/device/update/", deviceLocationHandler)
	http.HandleFunc("/device/lost/", deviceLostHandler)

	// device management
	http.HandleFunc("/device/list", deviceListHandler)
	http.HandleFunc("/device/add/", deviceAddHandler)

	// personas
	http.HandleFunc("/auth/check",    loginCheckHandler)
	http.HandleFunc("/auth/login",    loginHandler)
	http.HandleFunc("/auth/applogin", appLoginHandler)
	http.HandleFunc("/auth/logout",   logoutHandler)


	serveSingle("/",                      "./static/index.html")
	serveSingle("/index.html",            "./static/index.html")
	serveSingle("/install.html",          "./static/install.html")
	serveSingle("/push.html",             "./static/push.html")
	serveSingle("/app.html",              "./app/index.html")
	serveSingle("/style.css",             "./static/style.css")
	serveSingle("/style-app.css",         "./static/style-app.css")
	serveSingle("/style-common.css",      "./static/style-common.css")
	serveSingle("/logos/64.png",          "./static/logos/64.png")
	serveSingle("/logos/128.png",         "./static/logos/128.png")
	serveSingle("/img/persona-login.png", "./static/img/persona_sign_in_black.png")
	serveSingle("/lib/mustache.js",       "./static/lib/mustache.js")

	http.HandleFunc("/manifest.webapp", func(w http.ResponseWriter, r *http.Request) {
		filename := "./app/manifest.webapp"
		log.Println("serving manifest from " + filename);

		w.Header()["Content-Type"] = []string{"application/x-web-app-manifest+json"}
		http.ServeFile(w, r, filename)
	})

	log.Println("Listening on", gServerConfig.Hostname+":"+gServerConfig.Port)

	if gServerConfig.UseTLS {
		err = http.ListenAndServeTLS(gServerConfig.Hostname+":"+gServerConfig.Port,
			gServerConfig.CertFilename,
			gServerConfig.KeyFilename,
			nil)
	} else {
		log.Println("This is a really unsafe way to run the push server.  Really.  Don't do this in production.")
		err = http.ListenAndServe(gServerConfig.Hostname+":"+gServerConfig.Port, nil)
	}

	log.Println("Exiting... ", err)
	gDB.Close()
}
