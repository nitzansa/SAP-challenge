package main

import (
	"encoding/json"
	"github.com/fatih/structs"
	"github.com/go-resty/resty/v2"
	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/middleware/basicauth"
	"gopkg.in/yaml.v3"
	"os"
	"strconv"

	osb "sigs.k8s.io/go-open-service-broker-client/v2"
)

func main() {
	app := iris.New()

	auth := basicauth.Default(map[string]string{
		"admin": "admin",
	})

	brokerAPIs := app.Party("/v2")
	{
		app.Use(auth)
		brokerAPIs.Get("/catalog", auth, GetCatalog)
		brokerAPIs.Get("/service_instances/{instance_id:string}/service_bindings/{binding_id:string}", auth, GetServiceBinding)
		brokerAPIs.Put("/service_instances/{instance_id:string}", auth, CreateServiceInstance)
		brokerAPIs.Put("/service_instances/{instance_id:string}/service_bindings/{binding_id:string}", auth, CreateServiceBinding)
	}
	serverPort := ":" + os.Getenv("PORT")
	app.Listen(serverPort)
	//app.Listen(":8080")
}
var instances_MAP = make(map[string]Credentials)
var biding_MAP = make(map[string]interface{})

//func routeHandler(ctx iris.Context) {
//	username, password, _ := ctx.Request().BasicAuth()
//	// [...]
//}

type Credentials struct {
	Connection_string string `json:"connection_string"`
	Device_type string `json:"device_type"`
	Id string `json:"id"`
	State string `json:"state"`
}

type Item struct {
	Device_type string `json:"device_type"`
	Id string `json:"id"`
	Seq int `json:"seq"`
	State string `json:"state"`
}

type Devices struct {
	Pages int `json:"pages"`
	Num_items int `json:"num_items"`
	Items []Item `json:"items"`
}

type DeviceStatus struct {
	Active_time int `json:"active_time"`
	Connection_string string `json:"connection_string"`
 	Cpu int `json:"cpu"`
	Device_type string `json:"device_type"`
	Id string `json:"id"`
	Seq int `json:"seq"`
	State string `json:"state"` //Online, Offline, Faulty
	Temperature int `json:"temperature"` //Overheating: device with a temperature > 29°C
}

func getJson(url string, target interface{}) error {
	client := resty.New()
	resp, err := client.R().SetHeader("Cookie","app=MTYxMzkwNjA2OXxEdi1CQkFFQ180SUFBUkFCRUFBQV9fcl9nZ0FDQm5OMGNtbHVad3dSQUE5emFXZHVYM1Z3WDJSbGRHRnBiSE1HYzNSeWFXNW5EUC1zQVAtcGV5SmxiV0ZwYkY5aFpHUnlaWE56SWpvaWJtbDBlbUZ1TG5OaFltRm5RSE5oY0M1amIyMGlMQ0ptYVhKemRGOXVZVzFsSWpvaVRtbDBlbUZ1SWl3aWJHRnpkRjl1WVcxbElqb2lVMkZpWVdjaUxDSnZZMk4xY0dGMGFXOXVJam9pSWl3aWRHVnNJam9pSWl3aWRXNXBkbVZ5YzJsMGVTSTZJaUlzSW5kdmNtdHdiR0ZqWlNJNklpSXNJbTltWm1WeWFXNW5jeUk2Wm1Gc2MyVXNJblJsY20xeklqcDBjblZsZlFaemRISnBibWNNQ2dBSWRYTmxjbTVoYldVR2MzUnlhVzVuREFvQUNHNXBkSHBoYm5OaHy3cOpifs3yTCEdrA73A41lpMEz0Wzk9BXN2uaJkvw5Dg==").
		SetHeader("Accept","application/json").
		SetResult(target).
		ForceContentType("application/json").
		Get(url)

	if err != nil {
		return err
	}

	return resp.RawBody().Close()
}

func getDevices(url string, ch chan []Item) {
	for i := 0; i < 100; i++ {
		page_info := new(Devices)
		getJson(url + strconv.Itoa(i), page_info)
		ch <- page_info.Items
	}

	close(ch)
}

func getOnlineDevices(items []Item, ch chan string, device_type string) {
	for i := 0; i < 10; i++ {
		if items[i].State == "Online" && items[i].Device_type == device_type{
			ch <- items[i].Id
			//fmt.Println("successfully wrote", items[i].Id, "to ch")
		}
	}

	close(ch)
}

func CreateServiceInstance(ctx iris.Context) {
	provisionDetails := &osb.ProvisionRequest{}
	instanceID := ctx.Params().Get("instance_id")

	reqBody, _ := ctx.GetBody()
	json.Unmarshal(reqBody, provisionDetails)

	c := make(chan []Item, 100)
	go getDevices("https://welcome.cfapps.us10.hana.ondemand.com/device?next=", c)
	for val := range c {
		ch := make(chan string, 10)
		go getOnlineDevices(val, ch, provisionDetails.Parameters["device_type"].(string))
		for v := range ch {
			device_info := new(DeviceStatus)
			getJson("https://welcome.cfapps.us10.hana.ondemand.com/device/" + v + "/status", device_info)
			if device_info.Temperature <= 29 {
				instances_MAP[instanceID] = Credentials{
					Connection_string: device_info.Connection_string,
					Device_type: device_info.Device_type,
					Id: device_info.Id,
					State: device_info.State}
				ctx.JSON(&osb.ProvisionResponse{})
				return
			}
		}
	}

	ctx.JSON(&osb.ProvisionResponse{})
}

func GetServiceBinding(ctx iris.Context) {
	instanceID := ctx.Params().Get("instance_id")
	//bidingID := ctx.Params().Get("binding_id")
	ctx.JSON(&osb.GetBindingResponse{
		Credentials:     structs.Map(instances_MAP[instanceID]),
	})
}

type Biding struct {
	Biding_id	string
	Instance_id string
	//Credential	Credentials
}

func CreateServiceBinding(ctx iris.Context) {
	instanceID := ctx.Params().Get("instance_id")
	bidingID := ctx.Params().Get("binding_id")
	biding := &Biding{
		Biding_id:      bidingID,
		Instance_id:	instanceID,
		//Credential: 	instances_MAP[instanceID],
	}
	biding_MAP[bidingID] = biding
	ctx.JSON(&osb.BindResponse{
		Credentials:     structs.Map(instances_MAP[instanceID]), //structs.Map(foundItem)
	})
}

func GetCatalog(ctx iris.Context) {
	response := &osb.CatalogResponse{}
	data := `
---
services:
- name: your-iot-service
 id: 4f6e6cf6-ffdd-425f-a2c7-3c9258ad246e-nitzansa
 description: The example service!
 bindable: true
 metadata:
   displayName: "Example service"
   imageUrl: https://avatars2.githubusercontent.com/u/19862012?s=200&v=4
 plans:
 - name: iot-service
   id: iot-service-nitzansa
   description: The default plan for the service
   free: true
`
	yaml.Unmarshal([]byte(data), &response)
	ctx.JSON(response)
	println(ctx.JSON(response))
}