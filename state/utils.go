package state

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha512"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/dropbox/godropbox/errors"
	"github.com/pritunl/pritunl-link/config"
	"github.com/pritunl/pritunl-link/errortypes"
	"github.com/pritunl/pritunl-link/utils"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

var (
	transport = &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	client = &http.Client{
		Transport: transport,
		Timeout:   5 * time.Second,
	}
)

type stateData struct {
	PublicAddress string   `json:"public_address"`
	Tunnels       int      `json:"tunnels"`
	Errors        []string `json:"errors"`
}

func GetState(uri string) (state *State, err error) {
	uriData, err := url.ParseRequestURI(uri)
	if err != nil {
		err = &errortypes.ParseError{
			errors.Wrap(err, "state: Failed to parse uri"),
		}
		return
	}

	data := &stateData{
		PublicAddress: config.Config.PublicAddress,
		Tunnels:       1,
	}
	dataBuf := &bytes.Buffer{}

	err = json.NewEncoder(dataBuf).Encode(data)
	if err != nil {
		err = &errortypes.ParseError{
			errors.Wrap(err, "state: Failed to parse request data"),
		}
		return
	}

	req, err := http.NewRequest(
		"PUT",
		fmt.Sprintf("https://%s/link/state", uriData.Host),
		dataBuf,
	)
	if err != nil {
		err = &errortypes.RequestError{
			errors.Wrap(err, "state: Request init error"),
		}
		return
	}

	req.Header.Set("Content-Type", "application/json")

	hostId := uriData.User.Username()
	hostSecret, _ := uriData.User.Password()
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	nonce := utils.RandStr(32)

	authStr := strings.Join([]string{
		hostId,
		timestamp,
		nonce,
		"PUT",
		"/link/state",
	}, "&")

	hashFunc := hmac.New(sha512.New, []byte(hostSecret))
	hashFunc.Write([]byte(authStr))
	rawSignature := hashFunc.Sum(nil)
	sig := base64.StdEncoding.EncodeToString(rawSignature)

	req.Header.Set("Auth-Token", hostId)
	req.Header.Set("Auth-Timestamp", timestamp)
	req.Header.Set("Auth-Nonce", nonce)
	req.Header.Set("Auth-Signature", sig)

	res, err := client.Do(req)
	if err != nil {
		err = &errortypes.RequestError{
			errors.Wrap(err, "state: Request put error"),
		}
		return
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		err = &errortypes.RequestError{
			errors.Wrapf(err, "state: Bad status %n code from server",
				res.StatusCode),
		}
		return
	}

	state = &State{}

	err = json.NewDecoder(res.Body).Decode(state)
	if err != nil {
		err = &errortypes.ParseError{
			errors.Wrap(err, "state: Failed to parse response data"),
		}
		return
	}

	return
}