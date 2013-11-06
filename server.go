package main

import (
	"flag"
	"fmt"
	"github.com/emicklei/go-restful"
	"go/build"
	"log"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"
)

var gDB *DB
var gPersona PersonaHandler
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

func serveIndexHtml(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, path.Join(gServerConfig.PackagePath, "static", "index.html"))
}

func ensureIsLoggedIn(request *restful.Request, response *restful.Response, chain *restful.FilterChain) {
	if !gPersona.IsLoggedIn(request.Request) {
		response.WriteErrorString(http.StatusUnauthorized, "Not logged in")
		return
	}

	chain.ProcessFilter(request, response)
}

func getDeviceForRequest(request *restful.Request, response *restful.Response) *Device {
	id, err := strconv.ParseInt(request.PathParameter("device-id"), 10, 64)
	if err != nil {
		response.WriteErrorString(http.StatusBadRequest, "Failed to parse device")
		return nil
	}

	device, err := gDB.GetDeviceById(id)
	if device != nil && device.User == gPersona.GetLoginName(request.Request) {
		return device
	}

	response.WriteErrorString(http.StatusNotFound, "Device not found")
	return nil
}

func addDevice(request *restful.Request, response *restful.Response) {
	indevice := new(Device)
	request.ReadEntity(indevice)

	name := indevice.Name
	endpoint := indevice.Endpoint

	if name == "" || endpoint == "" {
		response.WriteErrorString(http.StatusBadRequest, "No name or endpoint")
		return
	}

	device, err := gDB.AddDevice(gPersona.GetLoginName(request.Request), name, endpoint)
	if err == nil {
		response.WriteEntity(*device)
	} else {
		response.WriteErrorString(http.StatusInternalServerError, "Failed to add device")
	}
}

func serveDevicesByUser(request *restful.Request, response *restful.Response) {
	devices, _ := gDB.ListDevicesForUser(gPersona.GetLoginName(request.Request))

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
		response.WriteErrorString(http.StatusBadRequest, "No device in request")
		return
	}

	commands, err := gDB.ListCommandsForDevice(device)
	if err != nil {
		response.WriteErrorString(http.StatusBadRequest, "Failed to retrieve commands")
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
		response.WriteErrorString(http.StatusBadRequest, "Failed to parse commands")
		return
	}

	if err := gDB.UpdateCommandsForDevice(device.Id, commands); err != nil {
		response.WriteErrorString(http.StatusInternalServerError, "Failed to update commands")
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
		response.WriteErrorString(http.StatusBadRequest, "Failed to parse latitude")
	}

	longitude, err := strconv.ParseFloat(request.QueryParameter("longitude"), 64)
	if err != nil {
		response.WriteErrorString(http.StatusBadRequest, "Failed to parse longitude")
	}

	err = gDB.UpdateDeviceLocation(device, latitude, longitude)
	if err != nil {
		response.WriteErrorString(http.StatusInternalServerError, "Failed to update location")
	}
}

func serveInvocation(request *restful.Request, response *restful.Response) {
	token, err := strconv.ParseInt(request.PathParameter("token"), 10, 64)
	if err != nil {
		response.WriteErrorString(http.StatusBadRequest, "Failed to parse invocation")
		return
	}

	// TODO check whether invocation was actually intended for device? how?
	context, exists := gPendingCommands[token]
	if !exists {
		response.WriteErrorString(http.StatusBadRequest, "Failed to find invocation")
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
		response.WriteErrorString(http.StatusBadRequest, "Failed to parse command")
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
		response.WriteErrorString(http.StatusBadRequest, "No such command for device")
		return
	}

	token := int64(time.Now().Unix())
	context := CommandContext{CommandId: cmdid}

	// Store pending arguments, if any
	if request.Request.ContentLength != 0 {
		if err = request.ReadEntity(&context.Arguments); err != nil {
			response.WriteErrorString(http.StatusBadRequest, "Failed to parse arguments")
			return
		}
	}
	gPendingCommands[token] = context

	// Issue push notification to device
	body := fmt.Sprintf("version=%d", token)
	pushRequest, err := http.NewRequest("PUT", device.Endpoint, strings.NewReader(body))
	if err != nil {
		response.WriteErrorString(http.StatusInternalServerError, "Failed to push command")
		return
	}

	pushRequest.Header["Content-Type"] = []string{"application/x-www-form-urlencoded"}

	var client http.Client
	_, err = client.Do(pushRequest)
	if err != nil {
		response.WriteErrorString(http.StatusInternalServerError, "Failed to push command")
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
		Consumes("application/json").
		Doc("Add a device").
		Param(ws.QueryParameter("name", "The name for the device")).
		Param(ws.QueryParameter("endpoint", "The push endpoint for the device")).
		Writes(Device{}))

	ws.
		Route(ws.POST("/location/{device-id}").To(updateDeviceLocation).
		Consumes("application/x-www-form-urlencoded").
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
		Consumes("application/json").
		Doc("Update the list of commands available for a device").
		Param(ws.PathParameter("device-id", "The identifier for the device")).
		Param(ws.QueryParameter("commands", "List of command ids supported by the device")))

	ws.
		Route(ws.POST("/{device-id}/command/{command-id}").To(triggerCommand).
		Consumes("application/json").
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

func setupPersonaHandlers() {
	gPersona = NewPersonaHandler(gServerConfig.PersonaName, gServerConfig.SessionCookie)
	http.HandleFunc("/auth/login", makePersonaLoginHandler("https://verifier.login.persona.org/verify"))
	http.HandleFunc("/auth/applogin", makePersonaLoginHandler("https://firefoxos.persona.org/verify"))
	http.HandleFunc("/auth/logout", gPersona.Logout)

	http.HandleFunc("/manifest.webapp", func(w http.ResponseWriter, r *http.Request) {
		filename := "./app/manifest.webapp"
		log.Println("serving manifest from " + filename)

		w.Header()["Content-Type"] = []string{"application/x-web-app-manifest+json"}
		http.ServeFile(w, r, filename)
	})
}

func setupStaticHandlers(packagePath string) {
	http.HandleFunc("/", serveIndexHtml)
	http.HandleFunc("/index.html", serveIndexHtml)
	log.Println(path.Join(packagePath, "static"))
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(path.Join(packagePath, "static")))))
}

func defaultBase(path string) string {
	p, err := build.Default.Import(path, "", build.FindOnly)
	log.Println(p)
	if err != nil {
		return "."
	}
	return p.Dir
}

// Persona's verifier URL is different for Firefox OS and Firefox Desktop
func makePersonaLoginHandler(verifierURL string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		err := gPersona.Login(verifierURL, w, r)
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(err.Error()))
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}
}

func main() {
	var packagePath = defaultBase("github.com/dougt/whereismyfox")
	var configFile = flag.String("config", path.Join(packagePath, "config.json"), "Location of configuration file")
	var dbFile = flag.String("db", path.Join(packagePath, "db.sqlite"), "Location of database")
	flag.Parse()

	readConfig(*configFile)
	gServerConfig.PackagePath = packagePath

	db, err := OpenDB(*dbFile)
	if err != nil {
		panic(err)
	}

	gDB = db
	if err = populateCommandsDB(db, path.Join(packagePath, "commands.json")); err != nil {
		panic(err)
	}

	gPendingCommands = map[int64]CommandContext{}

	restful.Add(createDeviceWebService())
	setupPersonaHandlers()
	setupStaticHandlers(packagePath)

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
