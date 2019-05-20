package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"runtime"
	"strings"

	"github.com/urfave/cli"
)

// AuthCodeService is service to perform authcode functions
type AuthCodeService struct {
	ClientID     string `json:"ClientID"`
	ClientSecret string `json:"ClientSecret"`
	Redirect     string `json:"Redirect"`
	Port         string `json:"Port"`
	RootURL      string `json:"RootURL"`
	authCode     string
	accessToken  string
	refreshToken string
}

// AuthCodeResponse is the basic response from WSO2
type AuthCodeResponse struct {
	RefreshToken string `json:"refresh_token"`
	AccessToken  string `json:"access_token"`
}

func main() {
	app := cli.NewApp()
	app.Name = "AuthCode Test"
	app.Usage = "Test authcode flow and get tokens from authcode and refresh token"
	app.Version = "0.1.0"
	app.Commands = []cli.Command{
		{
			Name:    "test",
			Aliases: []string{"t"},
			Usage:   "run authcode flow test",
			Action:  cliTest,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "config, c",
					Value: getDefaultFile(),
					Usage: "Load configuration from `FILE`",
				},
			},
		},
		{
			Name:    "generate",
			Aliases: []string{"gen", "g"},
			Usage:   "Generate config file",
			Action:  cliGenerate,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "output, o",
					Value: getDefaultFile(),
					Usage: "Output to `FILE`",
				},
			},
		},
	}
	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

func cliTest(c *cli.Context) error {
	configFile := c.String("config")
	authService := newAuthCodeServiceFromFile(configFile)
	authService.runAuthTest()
	return nil
}

func newAuthCodeServiceFromFile(configFile string) *AuthCodeService {
	file, err := ioutil.ReadFile(configFile)
	if err != nil {
		log.Fatalf("Error reading config file: %v", err)
	}
	authService := &AuthCodeService{}

	err = json.Unmarshal(file, authService)
	if err != nil {
		log.Fatalf("Error parsing config file: %v", err)
	}
	return authService
}

func getDefaultFile() string {
	user, err := user.Current()
	if err != nil {
		log.Fatalf("Coulding get current user: %v", err)
	}
	return fmt.Sprintf("%s/.authcodetest", user.HomeDir)
}

func cliGenerate(c *cli.Context) error {
	var clientID, clientSecret, redirect, apiURL string
	fmt.Print("Client ID: ")
	fmt.Scanln(&clientID)
	fmt.Print("Client Secret: ")
	fmt.Scanln(&clientSecret)
	fmt.Print("Redirect URL: ")
	fmt.Scanln(&redirect)
	fmt.Print("Root API URL: ")
	fmt.Scanln(&apiURL)

	u, _ := url.Parse(redirect)

	config := &AuthCodeService{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Redirect:     redirect,
		Port:         u.Port(),
		RootURL:      apiURL,
	}
	file, _ := json.MarshalIndent(config, "", "  ")
	file = append(file, '\n')
	fmt.Printf("%s", file)
	outfile := c.String("output")
	err := ioutil.WriteFile(outfile, file, 0644)
	if err != nil {
		log.Fatalf("Couldn't write config file: %v", err)
	}
	return nil
}

func (a *AuthCodeService) runAuthTest() {
	a.getAuthCode()
	err := a.getToken("authcode")
	if err != nil {
		log.Fatal("Error getting token from auth code")
	}
	log.Printf("Returned Auth - Refresh: %s - AccessToken: %s", a.refreshToken, a.accessToken)
	log.Print("Calling Echo...")
	echo := a.callEcho()
	log.Printf("Echo Status: %t", echo)
	log.Print("Refreshing Token...")
	err = a.getToken("refresh")
	if err != nil {
		log.Fatal("Error refreshing token")
	}
	log.Printf("Returned Refresh: %s", a.refreshToken)
	log.Print("Refreshing Token...")
	err = a.getToken("refresh")
	if err != nil {
		log.Fatal("Error refreshing token")
	}
	log.Printf("Returned Refresh: %s", a.refreshToken)
}

func (a *AuthCodeService) getAuthCode() {
	codeChan := make(chan string)
	go startListener(a.Port, codeChan)
	url := fmt.Sprintf("%s/authorize?response_type=code&client_id=%s&redirect_uri=%s&scope=PRODUCTION", a.RootURL, a.ClientID, a.Redirect)
	if runtime.GOOS == "darwin" {
		log.Printf("Launching Redirect URL in background: %s ", url)
		cmd := exec.Command("/usr/bin/open", "-g", url)
		cmd.Start()
	} else {
		fmt.Printf("Login URL: %s", url)
	}
	a.authCode = <-codeChan
	log.Print("Auth Code:", a.authCode)
}

func startListener(port string, c chan<- string) {
	port = fmt.Sprintf(":%s", port)
	stop := make(chan bool)
	srv := http.Server{Addr: port}
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		code, ok := r.URL.Query()["code"]
		if !ok {
			log.Print("Auth Code not found")
		}
		io.WriteString(w, "success")
		c <- code[len(code)-1]
		stop <- true
	})
	log.Print("Starting localhost listener")
	log.Print("If waiting check browser window for CAS")
	go srv.ListenAndServe()
	<-stop
	srv.Close()
}

func (a *AuthCodeService) getToken(method string) error {
	tokenURL := fmt.Sprintf("%s/token", a.RootURL)
	data := url.Values{}
	if method == "authcode" {
		data.Set("grant_type", "authorization_code")
		data.Set("code", a.authCode)
		data.Set("redirect_uri", a.Redirect)
	} else if method == "refresh" {
		data.Set("grant_type", "refresh_token")
		data.Set("refresh_token", a.refreshToken)
	} else {
		return errors.New("Invalid Method")
	}
	client := &http.Client{}
	req, _ := http.NewRequest("POST", tokenURL, strings.NewReader(data.Encode()))
	req.SetBasicAuth(a.ClientID, a.ClientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	var response AuthCodeResponse
	if resp.StatusCode == 200 {
		json.Unmarshal(body, &response)
		a.accessToken = response.AccessToken
		a.refreshToken = response.RefreshToken
		return nil
	}
	log.Printf("Token Response Code: %d", resp.StatusCode)
	log.Printf("ERROR token body: %s", body)
	return errors.New("Invalid response")
}

func (a *AuthCodeService) callEcho() bool {
	echoURL := fmt.Sprintf("%s/echo/v2/ping", a.RootURL)
	client := &http.Client{}
	req, _ := http.NewRequest("GET", echoURL, nil)
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", a.accessToken))
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	if resp.StatusCode == 200 {
		return true
	}
	log.Printf("ERROR: Echo status Code: %d", resp.StatusCode)
	log.Printf("ERROR: Echo body %s", body)
	return false
}
