package exttestutils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	dac "github.com/mongodb-forks/digest"
)

var usedRegions string
var pvtEpMongoDbId interface{}

type PvtEpMongoReq struct {
	ProviderName string `json:"providerName"`
	Region       string `json:"region"`
}

type PvtEpMongoReqConfig struct {
	ID        string `json:"id"`
	IPAddress string `json:"privateEndpointIPAddress"`
}

const (
	Azure                = "AZURE"
	PvtEpInitiatingState = "INITIATING"
	PvtEpAlreadyExists   = "PRIVATE_ENDPOINT_SERVICE_ALREADY_EXISTS_FOR_REGION"
)

func CreatePvtEpFromMongoDb(host string, publicKey string, privateKey string) (int, error) {

	// Available Regions from MongoDB Atlas
	regions := []string{"centralus", "eastus", "eastus2", "westus", "westus2"}

	// Choose a region that hasn't been used before
	var region string
	for i := 0; i < len(regions); i++ {
		region = regions[i]
		if region == usedRegions {
			continue
		}

		payload := &PvtEpMongoReq{
			ProviderName: Azure,
			Region:       region,
		}

		//Convert payload type to bytes
		var buf bytes.Buffer
		err := json.NewEncoder(&buf).Encode(payload)
		if err != nil {
			return 0, err
		}

		test := dac.NewTransport(publicKey, privateKey)

		//Make a http request to create private ep from mongoDB
		req, err := http.NewRequest(http.MethodPost, host+"/privateEndpoint/endpointService", &buf)

		if err != nil {
			return 0, err
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")

		resp, err := test.RoundTrip(req)
		if err != nil {
			return 0, err
		}
		defer func() {
			if resp != nil && resp.Body != nil {
				resp.Body.Close()
			}
		}()

		if resp.StatusCode != http.StatusOK {
			return resp.StatusCode, fmt.Errorf("expected status code %d, but got %d", http.StatusOK, resp.StatusCode)
		}

		if resp.Body != nil {
			bytes, err := io.ReadAll(resp.Body)
			if err != nil {
				return 0, err
			}

			fmt.Println(string(bytes))

			var resp map[string]interface{}
			if err := json.Unmarshal(bytes, &resp); err != nil {
				return 0, err
			}
			pvtEpMongoDbId = resp["id"]

			if resp["errorCode"] == PvtEpAlreadyExists {
				usedRegions = region
			}
			if resp["status"] == PvtEpInitiatingState {
				fmt.Printf("Created Private Endpoint in MongoDB Atlas in %s region, Waiting for 3 mins to generate Resource ID\n", region)
				time.Sleep(3 * time.Minute)
				//Get Resource ID from MongoDB Atlas
				resourceIDMongo := GetResourceIDfromMongoDb(host, publicKey, privateKey, pvtEpMongoDbId)
				if resourceIDMongo == "" {
					return 0, fmt.Errorf("error in fetching resourceID. Stopping tests")
				}
				return http.StatusOK, nil

			}

		}
		// Check if the current region is the last index of the regions
		if i == len(regions)-1 {
			return 0, fmt.Errorf("All regions are used, Delete All Private Endpoints manually from MongoDb Atlas and try again")
		}
	}

	return 0, nil
}

func ConfigurePvtEpFromMongoDb(host string, publicKey string, privateKey string, ipAddress string, resourceIDSaas string, pvtEpMongoDbIdStr string) (int, error) {

	payload := &PvtEpMongoReqConfig{
		ID:        resourceIDSaas,
		IPAddress: ipAddress,
	}

	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(payload)
	if err != nil {
		log.Fatal(err)
	}

	//host := data.MongoDbUrl
	test := dac.NewTransport(publicKey, privateKey)

	//Make a http request to create using Resource ID and IP address from Saas ENV
	req, err := http.NewRequest(http.MethodPost, host+"/privateEndpoint/AZURE/endpointService/"+pvtEpMongoDbIdStr+"/endpoint", &buf)
	if err != nil {
		log.Fatalln(err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := test.RoundTrip(req)
	if err != nil {
		log.Fatalln(err)
	}

	defer func() {
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
	}()

	if resp.Body != nil {
		bytes, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Fatal(err)
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(bytes, &resp); err != nil {
			log.Fatal(err)
		}

		if resp["status"] == PvtEpInitiatingState {
			fmt.Printf("Private Endpoint is being Configured in MongoDB Atlas\n")
			return http.StatusOK, err
		} else {
			log.Fatal("Error in Configuring Private EP, check MongoDB Atlas. Exiting tests\n")
		}

	}
	return 0, err
}

func GetResourceIDfromMongoDb(host string, publicKey string, privateKey string, id interface{}) string {

	var resourceIDStr string

	//host := data.MongoDbUrl
	test := dac.NewTransport(publicKey, privateKey)

	//Make a http request to create private ep from mongoDB
	req, err := http.NewRequest(http.MethodGet, host+"/privateEndpoint/AZURE/endpointService", nil)
	if err != nil {
		log.Fatalln(err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := test.RoundTrip(req)
	if err != nil {
		log.Fatalln(err)
	}

	defer func() {
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
	}()

	if resp.Body != nil {
		bytes, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Fatal(err)
		}

		var respMap []map[string]interface{}
		if err := json.Unmarshal(bytes, &respMap); err != nil {
			log.Fatal(err)
		}

		for _, resp := range respMap {
			if resp["id"] == id {
				resourceID := resp["privateLinkServiceResourceId"]
				resourceIDStr = resourceID.(string)
				return resourceIDStr
			}

		}
	}
	return ""
}
