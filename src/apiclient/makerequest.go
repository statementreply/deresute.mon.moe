package apiclient

import (
	"net/http"
	"crypto/sha1"
	"crypto/md5"
	"fmt"
	"encoding/hex"
	"io/ioutil"
	"strings"
)

func (client *ApiClient) MakeRequest(path, body string) *http.Request {
	var req *http.Request
	if client.sid == "" {
		client.sid = client.viewer_id_str + client.udid
	}
	param_tmp := sha1.Sum([]byte(client.udid + client.viewer_id_str + path + client.plain))
	sid_tmp := md5.Sum([]byte(client.sid + string(client.SID_KEY)))
	device_id_tmp := md5.Sum([]byte("Totally a real Android"))
	headers := map[string]string{
		"PARAM":           hex.EncodeToString(param_tmp[:]),
		"KEYCHAIN":        "",
		"USER_ID":         Lolfuscate(fmt.Sprintf("%d", client.user)),
		"CARRIER":         "google",
		"UDID":            Lolfuscate(client.udid),
		"APP_VER":         "2.0.3",
		"RES_VER":         client.res_ver,
		"IP_ADDRESS":      "127.0.0.1",
		"DEVICE_NAME":     "Nexus 42",
		"X-Unity-Version": "5.1.2f1",
		"SID":             hex.EncodeToString(sid_tmp[:]),
		"GRAPHICS_DEVICE_NAME": "3dfx Voodoo2 (TM)",
		"DEVICE_ID":            hex.EncodeToString(device_id_tmp[:]),
		"PLATFORM_OS_VERSION":  "Android OS 13.3.7 / API-42 (XYZZ1Y/74726f6c6c)",            "DEVICE":               "2",
		"Content-Type":         "application/x-www-form-urlencoded", // lies
		"User-Agent":           "Dalvik/2.1.0 (Linux; U; Android 13.3.7; Nexus 42 Build/XYZZ1Y)",              "Accept-Encoding":      "identity",
		"Connection":           "close",
	}
	// Request header ready

	// Prepare Request struct
	// req.body is ReadCloser
	req, _ = http.NewRequest("POST", BASE+path, ioutil.NopCloser(strings.NewReader(body)))
	for k := range headers {
		req.Header.Set(k, headers[k])
		// not needed
		//req.Header.Set(http.CanonicalHeaderKey(k), headers[k])
	}
	req.Close = true
	return req
}
