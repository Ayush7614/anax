package api

import (
	"encoding/json"
	"fmt"
	"github.com/golang/glog"
	"github.com/gorilla/mux"
	"github.com/open-horizon/anax/eventlog"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/version"
	"io/ioutil"
	"net/http"
)

type managementStatusInput struct {
	Type         string `json:"type,omitempty"`
	Status       string `json:"status,omitempty"`
	ErrorMessage string `json:"errorMessage,omitempty"`
}

func (a *API) managementStatus(w http.ResponseWriter, r *http.Request) {

	resource := "management"
	errorHandler := GetHTTPErrorHandler(w)

	switch r.Method {
	case "GET":
		glog.V(5).Infof(apiLogString(fmt.Sprintf("Handling %v on resource %v", r.Method, resource)))

		pathVars := mux.Vars(r)
		nmpName := pathVars["nmpname"]
		orgName := pathVars["org"]

		// Get the NMP status(es)
		if errHandled, out := FindManagementStatusForOutput(nmpName, orgName, errorHandler, a.db); errHandled {
			return
		} else {
			writeResponse(w, out, http.StatusOK)
		}

	case "PUT":
		glog.V(5).Infof(apiLogString(fmt.Sprintf("Handling %v on resource %v", r.Method, resource)))

		pathVars := mux.Vars(r)
		nmpName := pathVars["nmpname"]

		// Must include nmpname in URL
		if nmpName == "" {
			errorHandler(NewBadRequestError(fmt.Sprintf("path variable \"nmpname\" missing.")))
			return
		}

		// Read in the HTTP body.
		var nmStatus managementStatusInput
		body, _ := ioutil.ReadAll(r.Body)
		if err := json.Unmarshal(body, &nmStatus); err != nil {
			errorHandler(NewAPIUserInputError(fmt.Sprintf("Input body couldn't be deserialized to %v object: %v, error: %v", resource, string(body), err), "management status"))
			return
		}

		// Make sure current exchange version meet the requirement
		if err := version.VerifyExchangeVersion(a.GetHTTPFactory(), a.GetExchangeURL(), a.GetExchangeId(), a.GetExchangeToken(), false); err != nil {
			eventlog.LogExchangeEvent(a.db, persistence.SEVERITY_ERROR,
				persistence.NewMessageMeta(EL_API_ERR_IN_VERIFY_EXCH_VERSION, err.Error()),
				persistence.EC_EXCHANGE_ERROR, a.GetExchangeURL())
			errorHandler(NewSystemError(fmt.Sprintf("Error verifiying exchange version. error: %v", err)))
			return
		}

		// Create handler for putting updated NMP status in the exchange
		statusHandler := exchange.GetPutNodeManagementPolicyStatusHandler(a)

		// Update the NMP Status
		errHandled, out, msgs := UpdateManagementStatus(nmStatus, errorHandler, statusHandler, nmpName, a.db)
		if errHandled {
			return
		}

		// Send out all messages
		for _, msg := range msgs {
			a.Messages() <- msg
		}

		// Send out the config complete message that enables the device for agreements
		a.Messages() <- events.NewEdgeConfigCompleteMessage(events.NM_STATUS_CHANGED)

		writeResponse(w, out, http.StatusCreated)

	case "OPTIONS":
		w.Header().Set("Allow", "GET, PUT, OPTIONS")
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (a *API) nextUpgradeJob(w http.ResponseWriter, r *http.Request) {

	resource := "management"

	errorHandler := GetHTTPErrorHandler(w)

	switch r.Method {
	case "GET":
		glog.V(5).Infof(apiLogString(fmt.Sprintf("Handling %v on resource %v", r.Method, resource)))

		// Retrieve the optional query parameter
		jobType := r.URL.Query().Get("type")
		ready := r.URL.Query().Get("ready")

		// Get the next NMP Job Status
		if errHandled, out := FindManagementNextJobForOutput(jobType, ready, errorHandler, a.db); errHandled {
			return
		} else {
			writeResponse(w, out, http.StatusOK)
		}

	case "OPTIONS":
		w.Header().Set("Allow", "GET, OPTIONS")
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}
