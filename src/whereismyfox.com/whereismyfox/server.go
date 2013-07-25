package main

import (
	"fmt"
	"github.com/emicklei/go-restful"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var gDB *DB

func serveSingle(pattern string, filename string) {
	http.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		log.Println("serving file " + filename)
		http.ServeFile(w, r, filename)
	})
}

func ensureIsLoggedIn(request *restful.Request, response *restful.Response, chain *restful.FilterChain) {
	if !IsLoggedIn(request.Request) {
		response.WriteError(http.StatusUnauthorized, nil)
		return
	}

	chain.ProcessFilter(request, response)
}

func getDeviceForRequest(request *restful.Request, response *restful.Response) *Device {
	id, err := strconv.ParseInt(request.PathParameter("device-id"), 10, 64)
	if err != nil {
		response.WriteError(http.StatusBadRequest, nil)
		return nil
	}

	device, err := gDB.GetDeviceById(id)
	if device != nil && device.User == GetLoginName(request.Request) {
		return device
	}

	response.WriteError(http.StatusNotFound, nil)
	return nil
}

func addDevice(request *restful.Request, response *restful.Response) {
	indevice := new(Device)
	request.ReadEntity(indevice)

	name := indevice.Name
	endpoint := indevice.Endpoint

	if name == "" || endpoint == "" {
		response.WriteError(http.StatusBadRequest, nil)
		return
	}

	device, err := gDB.AddDevice(GetLoginName(request.Request), name, endpoint)
	if err == nil {
		response.WriteEntity(*device)
	} else {
		response.WriteError(http.StatusInternalServerError, nil)
	}
}

func serveDevicesByUser(request *restful.Request, response *restful.Response) {
	devices, _ := gDB.ListDevicesForUser(GetLoginName(request.Request))

	urls := []string{}
	for _, d := range devices {
		urls = append(urls, fmt.Sprintf("/device/%d", d.Id))
	}

	response.WriteEntity(urls)
}

func serveDevice(request *restful.Request, response *restful.Response) {
	if device := getDeviceForRequest(request, response); device != nil {
		response.WriteEntity(*device)
	}
}

func updateDeviceLocation(request *restful.Request, response *restful.Response) {
	device := getDeviceForRequest(request, response)
	if device == nil {
		return
	}

	latitude, err := strconv.ParseFloat(request.QueryParameter("latitude"), 64)
	if err != nil {
		response.WriteError(http.StatusBadRequest, nil)
	}

	longitude, err := strconv.ParseFloat(request.QueryParameter("longitude"), 64)
	if err != nil {
		response.WriteError(http.StatusBadRequest, nil)
	}

	err = gDB.UpdateDeviceLocation(device, latitude, longitude)
	if err != nil {
		response.WriteError(http.StatusInternalServerError, nil)
	}
}

func reportDeviceLost(request *restful.Request, response *restful.Response) {
	device := getDeviceForRequest(request, response)
	if device == nil {
		return
	}

	// Issue push notification to device
	body := fmt.Sprintf("version=%d", uint64(time.Now().Unix()))
	pushRequest, err := http.NewRequest("PUT", device.Endpoint, strings.NewReader(body))
	if err != nil {
		response.WriteError(http.StatusInternalServerError, nil)
	}

	pushRequest.Header["Content-Type"] = []string{"application/x-www-form-urlencoded"}

	var client http.Client
	_, err = client.Do(pushRequest)
	if err != nil {
		response.WriteError(http.StatusInternalServerError, nil)
	}
}

func createDeviceWebService() *restful.WebService {
	ws := new(restful.WebService)

	ws.
		Filter(ensureIsLoggedIn).
		Path("/device").
		Consumes(restful.MIME_JSON).
		Produces(restful.MIME_JSON)

	ws.
		Route(ws.GET("/").To(serveDevicesByUser).
		Doc("Retrieve all devices owned by a user").
		Writes([]Device{}))

	ws.
		Route(ws.GET("/{device-id}").To(serveDevice).
		Doc("Retrieve a device based on its id").
		Param(ws.PathParameter("device-id", "The identifier for the device")).
		Writes(Device{}))

	ws.
		Route(ws.PUT("/").To(addDevice).
		Consumes("application/json; charset=UTF-8").
		Doc("Add a device").
		Param(ws.QueryParameter("name", "The name for the device")).
		Param(ws.QueryParameter("endpoint", "The push endpoint for the device")).
		Writes(Device{}))

	ws.
		Route(ws.POST("/location/{device-id}").To(updateDeviceLocation).
		Consumes("application/x-www-form-urlencoded; charset=UTF-8").
		Doc("Update a device's latitude and longitude").
		Param(ws.QueryParameter("latitude", "The latitude where the device was observed")).
		Param(ws.QueryParameter("longitude", "The longitude where the device was observed")).
		Writes(Device{}))

	ws.
		Route(ws.POST("/lost/{device-id}").To(reportDeviceLost).
		Doc("Report a device as lost").
		Writes(Device{}))

	return ws
}

func main() {
	readConfig()
	db, err := OpenDB("db.sqlite")
	if err != nil {
		panic(err)
	}

	gDB = db
	restful.Add(createDeviceWebService())

	// Persona handling
	http.HandleFunc("/auth/check", loginCheckHandler)
	http.HandleFunc("/auth/login", loginHandler)
	http.HandleFunc("/auth/applogin", appLoginHandler)
	http.HandleFunc("/auth/logout", logoutHandler)

	serveSingle("/", "./static/index.html")
	serveSingle("/index.html", "./static/index.html")
	serveSingle("/install.html", "./static/install.html")
	serveSingle("/push.html", "./static/push.html")
	serveSingle("/app.html", "./app/index.html")
	serveSingle("/style.css", "./static/style.css")
	serveSingle("/style-app.css", "./static/style-app.css")
	serveSingle("/style-common.css", "./static/style-common.css")
	serveSingle("/logos/64.png", "./static/logos/64.png")
	serveSingle("/logos/128.png", "./static/logos/128.png")
	serveSingle("/img/persona-login.png", "./static/img/persona-login.png")
	serveSingle("/lib/mustache.js", "./static/lib/mustache.js")

	http.HandleFunc("/manifest.webapp", func(w http.ResponseWriter, r *http.Request) {
		filename := "./app/manifest.webapp"
		log.Println("serving manifest from " + filename)

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
		log.Println("This is a really unsafe way to run the server.  Really.  Don't do this in production.")
		err = http.ListenAndServe(gServerConfig.Hostname+":"+gServerConfig.Port, nil)
	}

	log.Println("Exiting... ", err)
	gDB.Close()
}
