/*
 * Copyright (c) 2013-2014, Jeremy Bingham (<jbingham@gmail.com>)
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"encoding/json"
	"github.com/ctdk/goiardi/acl"
	"github.com/ctdk/goiardi/actor"
	"github.com/ctdk/goiardi/container"
	//"github.com/ctdk/goiardi/user"
	"github.com/ctdk/goiardi/organization"
	"github.com/ctdk/goiardi/util"
	"github.com/gorilla/mux"
	"net/http"
)

// container handlers

// may have separate handlers for each kind of container, if warranted.
func containerHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	vars := mux.Vars(r)

	orgName := vars["org"]
	org, orgerr := organization.Get(orgName)
	if orgerr != nil {
		jsonErrorReport(w, r, orgerr.Error(), orgerr.Status())
		return
	}

	opUser, oerr := actor.GetReqUser(org, r.Header.Get("X-OPS-USERID"))
	if oerr != nil {
		jsonErrorReport(w, r, oerr.Error(), oerr.Status())
		return
	}

	containerName := vars["name"]

	con, cerr := container.Get(org, containerName)
	if cerr != nil {
		jsonErrorReport(w, r, cerr.Error(), cerr.Status())
		return
	}

	containerACL, gerr := acl.GetItemACL(org, con)
	if gerr != nil {
		jsonErrorReport(w, r, gerr.Error(), gerr.Status())
		return
	}

	response := make(map[string]interface{})

	switch r.Method {
	case "GET":
		if f, ferr := containerACL.CheckPerm("read", opUser); ferr != nil {
			jsonErrorReport(w, r, ferr.Error(), ferr.Status())
			return
		} else if !f {
			jsonErrorReport(w, r, "You do not have permission to do that", http.StatusForbidden)
			return
		}
	case "DELETE":
		if f, ferr := containerACL.CheckPerm("delete", opUser); ferr != nil {
			jsonErrorReport(w, r, ferr.Error(), ferr.Status())
			return
		} else if !f {
			jsonErrorReport(w, r, "You do not have permission to do that", http.StatusForbidden)
			return
		}
		err := containerACL.Delete()
		if err != nil {
			jsonErrorReport(w, r, err.Error(), err.Status())
			return
		}
		err = con.Delete()
		if err != nil {
			jsonErrorReport(w, r, err.Error(), err.Status())
			return
		}
	default:
		w.Header().Set("Allow", "GET, DELETE")
		jsonErrorReport(w, r, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	response["containername"] = con.Name
	response["containerpath"] = con.Name // might be something else
	// sometimes

	enc := json.NewEncoder(w)
	if err := enc.Encode(&response); err != nil {
		jsonErrorReport(w, r, err.Error(), http.StatusInternalServerError)
	}
}

func containerListHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	vars := mux.Vars(r)

	orgName := vars["org"]
	org, orgerr := organization.Get(orgName)
	if orgerr != nil {
		jsonErrorReport(w, r, orgerr.Error(), orgerr.Status())
		return
	}

	opUser, oerr := actor.GetReqUser(org, r.Header.Get("X-OPS-USERID"))
	if oerr != nil {
		jsonErrorReport(w, r, oerr.Error(), oerr.Status())
		return
	}

	containerACL, err := acl.GetContainerACL(org, "containers")
	if err != nil {
		jsonErrorReport(w, r, err.Error(), err.Status())
		return
	}
	if f, ferr := containerACL.CheckPerm("read", opUser); ferr != nil {
		jsonErrorReport(w, r, ferr.Error(), ferr.Status())
		return
	} else if !f {
		jsonErrorReport(w, r, "You do not have permission to do that", http.StatusForbidden)
		return
	}

	var response interface{}

	switch r.Method {
	case "GET":
		cList := container.GetList(org)
		rp := make(map[string]interface{})
		for _, c := range cList {
			conURL := util.JoinStr("/organizations/", org.Name, "/containers/", c)
			rp[c] = util.CustomURL(conURL)
		}
		response = rp
	case "POST":
		if f, ferr := containerACL.CheckPerm("create", opUser); ferr != nil {
			jsonErrorReport(w, r, ferr.Error(), ferr.Status())
			return
		} else if !f {
			jsonErrorReport(w, r, "You do not have permission to do that", http.StatusForbidden)
			return
		}
		cData, err := parseObjJSON(r.Body)
		if err != nil {
			jsonErrorReport(w, r, err.Error(), http.StatusBadRequest)
			return
		}
		var contName string
		var ok bool
		if contName, ok = cData["id"].(string); !ok {
			contName, ok = cData["containername"].(string)
		}
		if !ok {
			jsonErrorReport(w, r, "invalid container name", http.StatusBadRequest)
			return
		} else if contName == "" {
			jsonErrorReport(w, r, "container name missing", http.StatusBadRequest)
			return
		}
		cont, cerr := container.New(org, contName)
		if cerr != nil {
			jsonErrorReport(w, r, cerr.Error(), cerr.Status())
			return
		}
		cerr = cont.Save()
		if cerr != nil {
			jsonErrorReport(w, r, cerr.Error(), cerr.Status())
			return
		}
		// newly created containers are only accessible by the creator
		newACL, gerr := acl.GetItemACL(org, cont)
		if gerr != nil {
			jsonErrorReport(w, r, gerr.Error(), gerr.Status())
			return
		}
		gerr = newACL.CreatorOnly(opUser)
		if gerr != nil {
			jsonErrorReport(w, r, gerr.Error(), gerr.Status())
			return
		}
		w.WriteHeader(http.StatusCreated)
		rp := make(map[string]interface{})
		//rp["containername"] = cont.Name
		//rp["containerpath"] = cont.Name
		rp["uri"] = util.ObjURL(cont)
		response = rp
	default:
		w.Header().Set("Allow", "GET, POST")
		jsonErrorReport(w, r, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	enc := json.NewEncoder(w)
	if err := enc.Encode(&response); err != nil {
		jsonErrorReport(w, r, err.Error(), http.StatusInternalServerError)
	}
}