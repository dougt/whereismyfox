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
var gPendingCommands map[int64]CommandContext

type CommandContext struct {
	CommandId int64           `json: "commandid"`
	Arguments map[string]bool `json: "arguments"`
}

type CommandResponse struct {
	Name        string `json: "name"`
	Description string `json: "description"`
	Trigger     string `json: "trigger"`
}

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

func toCommandResponse(device *Device, command *Command) CommandResponse {
	trigger := fmt.Sprintf("/device/%d/command/%d", device.Id, command.Id)
	return CommandResponse{command.Name, command.Description, trigger}
}

func serveCommandsByDevice(request *restful.Request, response *restful.Response) {
	device := getDeviceForRequest(request, response)
	if device == nil {
		response.WriteError(http.StatusBadRequest, nil)
		return
	}

	commands, err := gDB.ListCommandsForDevice(device)
	if err != nil {
		response.WriteError(http.StatusBadRequest, nil)
		return
	}

	responses := make([]CommandResponse, len(commands))
	for i, cmd := range commands {
		responses[i] = toCommandResponse(device, cmd)
	}

	response.WriteEntity(responses)
}

func updateCommandsByDevice(request *restful.Request, response *restful.Response) {
	device := getDeviceForRequest(request, response)
	if device == nil {
		return
	}

	commands := []int64{}
	if err := request.ReadEntity(&commands); err != nil {
		response.WriteError(http.StatusBadRequest, nil)
		return
	}

	if err := gDB.UpdateCommandsForDevice(device.Id, commands); err != nil {
		response.WriteError(http.StatusInternalServerError, nil)
		return
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

func serveInvocation(request *restful.Request, response *restful.Response) {
	token, err := strconv.ParseInt(request.PathParameter("token"), 10, 64)
	if err != nil {
		response.WriteError(http.StatusBadRequest, nil)
		return
	}

	// TODO check whether invocation was actually intended for device? how?
	context, exists := gPendingCommands[token]
	if !exists {
		response.WriteError(http.StatusBadRequest, nil)
		return
	}

	delete(gPendingCommands, token)
	response.WriteEntity(context)
}

func triggerCommand(request *restful.Request, response *restful.Response) {
	device := getDeviceForRequest(request, response)
	if device == nil {
		return
	}

	cmdid, err := strconv.ParseInt(request.PathParameter("command-id"), 10, 64)
	if err != nil {
		response.WriteError(http.StatusBadRequest, nil)
		return
	}

	// Check whether the device actually implements the command
	implements := false

	commands, _ := gDB.ListCommandsForDevice(device)
	for _, cmd := range commands {
		if cmd.Id == cmdid {
			implements = true
			break
		}
	}

	if implements == false {
		response.WriteError(http.StatusBadRequest, nil)
		return
	}

	token := int64(time.Now().Unix())
	context := CommandContext{CommandId: cmdid}

	// Store pending arguments, if any
	if request.Request.ContentLength != 0 {
		if err = request.ReadEntity(&context.Arguments); err != nil {
			response.WriteError(http.StatusBadRequest, nil)
			return
		}
	}
	gPendingCommands[token] = context

	// Issue push notification to device
	body := fmt.Sprintf("version=%d", token)
	pushRequest, err := http.NewRequest("PUT", device.Endpoint, strings.NewReader(body))
	if err != nil {
		response.WriteError(http.StatusInternalServerError, nil)
		return
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
		Param(ws.QueryParameter("longitude", "The longitude where the device was observed")))

	ws.
		Route(ws.GET("/{device-id}/command").To(serveCommandsByDevice).
		Doc("List the commands available for a device").
		Param(ws.PathParameter("device-id", "The identifier for the device")).
		Writes([]CommandResponse{}))

	ws.
		Route(ws.PUT("/{device-id}/command").To(updateCommandsByDevice).
		Consumes("application/json; charset=UTF-8").
		Doc("Update the list of commands available for a device").
		Param(ws.PathParameter("device-id", "The identifier for the device")).
		Param(ws.QueryParameter("commands", "List of command ids supported by the device")))

	ws.
		Route(ws.POST("/{device-id}/command/{command-id}").To(triggerCommand).
		Consumes("application/json; charset=UTF-8").
		Doc("Trigger a command").
		Param(ws.PathParameter("device-id", "The identifier for the device")).
		Param(ws.PathParameter("command-id", "The identifier for the command")).
		Param(ws.QueryParameter("parameters", "An object with values for parameters")))

	// FIXME should this be under /device?
	ws.
		Route(ws.GET("/invocation/{token}").To(serveInvocation).
		Doc("Get the invocation context of a command").
		Param(ws.PathParameter("token", "The invocation identifier")).
		Writes(CommandContext{}))

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

	if err = populateCommandsDB(db, "commands.json"); err != nil {
		panic(err)
	}

	gPendingCommands = map[int64]CommandContext{}

	// Persona handling
	http.HandleFunc("/auth/check", loginCheckHandler)
	http.HandleFunc("/auth/login", loginHandler)
	http.HandleFunc("/auth/applogin", appLoginHandler)
	http.HandleFunc("/auth/logout", logoutHandler)

	serveSingle("/", "./static/index.html")
	serveSingle("/index.html", "./static/index.html")
	serveSingle("/install.html", "./static/install.html")
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
