// Copyright 2016 GUO Yixuan <culy.gyx@gmail.com>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License version 3 as
// published by the Free Software Foundation.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.
//
// Ported from the Python implementation in deresute.me
//     <https://github.com/marcan/deresuteme>
//     Copyright 2016-2017 Hector Martin <marcan@marcan.st>
//     Licensed under the Apache License, Version 2.0

package apiclient

import (
	"crypto/md5"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"strings"
)

// don't export this
func (client *ApiClient) makeRequest(path, body, plain_tmp string) *http.Request {
	var req *http.Request
	// impossible branch
	/*if client.sid == "" {
		client.sid = client.viewer_id_str + client.udid
	}*/
	param_tmp := sha1.Sum([]byte(client.udid + client.viewer_id_str + path + plain_tmp))
	sid_tmp := md5.Sum([]byte(client.sid + string(client.SID_KEY)))
	device_id_tmp := md5.Sum([]byte("Totally a real Android"))
	headers := map[string]string{
		"PARAM":           hex.EncodeToString(param_tmp[:]),
		"KEYCHAIN":        "",
		"USER_ID":         Lolfuscate(fmt.Sprintf("%d", client.user)),
		"CARRIER":         "google",
		"UDID":            Lolfuscate(client.udid),
		"APP_VER":         client.app_ver,
		"RES_VER":         client.res_ver,
		"IP_ADDRESS":      "127.0.0.1",
		"DEVICE_NAME":     "Nexus 42",
		"X-Unity-Version": "5.1.2f1",
		"SID":             hex.EncodeToString(sid_tmp[:]),
		"GRAPHICS_DEVICE_NAME": "3dfx Voodoo2 (TM)",
		"DEVICE_ID":            hex.EncodeToString(device_id_tmp[:]),
		"PLATFORM_OS_VERSION":  "Android OS 13.3.7 / API-42 (XYZZ1Y/74726f6c6c)",
		"DEVICE": "2",
		"Content-Type": "application/x-www-form-urlencoded", // lies
		"User-Agent":   "Dalvik/2.1.0 (Linux; U; Android 13.3.7; Nexus 42 Build/XYZZ1Y)", "Accept-Encoding": "identity",
		"Connection": "close",
	}
	// Request header ready

	// Prepare Request struct
	req, err := http.NewRequest("POST", BASE+path, strings.NewReader(body))
	if err != nil {
		log.Fatal("http.NewRequset", err)
	}
	for k := range headers {
		req.Header.Set(k, headers[k])
		// not needed
		//req.Header.Set(http.CanonicalHeaderKey(k), headers[k])
	}
	req.Close = true
	return req
}
