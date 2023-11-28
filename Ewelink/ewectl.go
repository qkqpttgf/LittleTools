package main
import (
	"bufio"
	"crypto/hmac"
    "crypto/sha256"
    "encoding/base64"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var dbFilePath string
var operateLight string
var targetLight string
var startWeb bool
var Server *http.Server

type EwelinkAPI struct {
	scheme string
	host string
	oauthToken string
	refreshToken string
	viewStatus string
}
var ewelinkapi EwelinkAPI
func apiInit() {
	ewelinkapi.scheme = "https://"
	ewelinkapi.host = "cn-apia.coolkit.cn"
	ewelinkapi.oauthToken = "/v2/user/oauth/token"
	ewelinkapi.refreshToken = "/v2/user/refresh"
	ewelinkapi.viewStatus = "/v2/device/thing/status"
}
type OauthApp struct {
	appID string
	appSecret string
	redirectUrl string
}
var oauthapp OauthApp
func appInit() {
	oauthapp.appID, _ = readConfig(1, "appID")
	oauthapp.appSecret, _ = readConfig(1, "appSecret")
	oauthapp.redirectUrl, _ = readConfig(1, "appRedirect")
}

var quit chan int
var accessToken string

func main() {
	conlog(passlog("Program Start") + "\n")
	parseCommandLine()
	apiInit()
	if validToken() {
		// token 有效
		//fmt.Print("valid token: ")
		//fmt.Println(accessToken)
		//a,_ := listDevices()
		//fmt.Println(a)
		dIDs, _ := readConfig(1, "deviceIDs")
		deviceIDs := strings.Split(dIDs, ",")
		for _, device := range deviceIDs {
			status, _ := getDeviceStatus(1, device)
			conlog("  " + device + ": " + status + "\n")
		}
		if operateLight != "" {
			turnLight(targetLight, operateLight)
		}
		if startWeb {
			startSrv()
			waitSYS()
			stopSrv()
		}
	} else {
		conlog(alertlog("no valid token, start to Oauth.") + "\n")
		startTmpSrv()
		waitOauth()
		stopSrv()
	}

	conlog(passlog("Program End") + "\n")
}

func parseCommandLine() {
	configFile := false
	turnLight1 := false
	softPath := ""
	for argc, argv := range os.Args {
		conlog(fmt.Sprintf("%d: %v\n", argc, argv))
		if argc == 0 {
			softPath = argv
			softPath = softPath[0:strings.LastIndex(softPath, "/")]
		}
		if argv == "web" {
			startWeb = true
			continue
		}
		if len(argv) > 4 {
			if argv[0:4] == "turn" {
				operateLight = argv[4:]
				turnLight1 = true
				continue
			}
		}
		if turnLight1 {
			targetLight = argv
			turnLight1 = false
			continue
		}
		if argv == "-config" || argv == "-c" {
			configFile = true
			continue
		}
		if configFile {
			dbFilePath = argv
			configFile = false
			continue
		}
	}
	if dbFilePath == "" {
		dbFilePath = softPath + "/config.db"
	}
}

func conlog(log string) {
	layout := "[2006-01-02 15:04:05.000] "
	strTime := (time.Now()).Format(layout)
	fmt.Print(strTime, log)
}
func alertlog(log string) string {
	return fmt.Sprintf("\033[91;5m%s\033[0m", log)
}
func passlog(log string) string {
	return fmt.Sprintf("\033[92;32m%s\033[0m", log)
}

func startSrv() {
	http.HandleFunc("/", route)
	Server = listenHttp("", 60575)
	if Server == nil {
		conlog("Server start faild\n")
	} else {
		conlog("Server started\n")
	}
}
func startTmpSrv() {
	http.HandleFunc("/", oauthroute)
	Server = listenHttp("", 60576)
	if Server == nil {
		conlog("Oauth Server start faild\n")
	} else {
		conlog("Oauth Server started\n")
		conlog("Please visit http://{your ip}:60576/ in a browser.\n")
		fmt.Println("Waiting...")
	}
}
func listenHttp(bindIP string, port int) *http.Server {
	conlog(fmt.Sprintf("starting http at %v:%d\n", bindIP, port))
	srv := &http.Server{Addr: fmt.Sprintf("%v:%d", bindIP, port), Handler: nil}
	srv1, err := net.Listen("tcp", fmt.Sprintf("%v:%d", bindIP, port))
	if err != nil {
		conlog(fmt.Sprint(err, "\n"))
		conlog(alertlog("http start faild") + "\n")
		return nil
	}
	go srv.Serve(srv1)
	return srv
}
func stopSrv() {
	var err error
	if Server != nil {
		err = Server.Close()
		if err != nil {
			conlog(fmt.Sprint(err, "\n"))
		} else {
			conlog("  http closed\n")
		}
	}
}
func waitSYS() {
	sysSignalQuit := make(chan os.Signal, 1)
	signal.Notify(sysSignalQuit, syscall.SIGINT, syscall.SIGTERM)
	<-sysSignalQuit
}
func waitOauth() {
	quit = make(chan int, 1)
	<-quit
}

func route(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	r.ParseForm()
	fmt.Println(r.TLS != nil, r.Host, r.URL)
	fmt.Println(r.Header)
	path := r.URL.Path
	fmt.Println("_" + path)
    query := r.URL.Query()
    fmt.Println(query.Get("a"))
	data := r.Form
	fmt.Println(data)
}

func oauthroute(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	r.ParseForm()
	//fmt.Println(r.TLS != nil, r.Host, r.URL)
	//fmt.Println(r.Header)
	//path := r.URL.Path
	//fmt.Println("_" + path)
    query := r.URL.Query()
    //fmt.Println(query.Get("a"))
	//data := r.Form
	//fmt.Println(data)
	//-------------------------
	if query.Get("code") != "" {
		// 有code
		conlog("  received a code\n")
		data := "{\"code\":\"" + query.Get("code") + "\", \"redirectUrl\":\"" + oauthapp.redirectUrl + "\", \"grantType\":\"authorization_code\"}"
		head := make(map[string]string)
		head["X-CK-Appid"] = oauthapp.appID
		head["Content-Type"] = "application/json"
		head["Authorization"] = "Sign " + ComputeHmac256(data, oauthapp.appSecret)
		head["Host"] = ewelinkapi.host

		res, err := curl("POST", ewelinkapi.scheme + ewelinkapi.host + ewelinkapi.oauthToken, data, head, false, false)
		if err != nil {
			htmlOutput(w, fmt.Sprint(err), 400, nil)
		} else {
			//res.StatusCode
			conlog("  get access token success\n")
			body := res.Body
			accessToken = readValueInString(string(body), "accessToken")
			atet := readValueInString(string(body), "atExpiredTime")
			rt := readValueInString(string(body), "refreshToken")
			rtet := readValueInString(string(body), "rtExpiredTime")
			conlog("  saving access token\n")
			saveConfig(1, "accessToken", accessToken)
			saveConfig(1, "atExpiredTime", atet)
			saveConfig(1, "refreshToken", rt)
			err := saveConfig(1, "rtExpiredTime", rtet)
			if err != nil {
				conlog("  saving access token faild\n")
				htmlOutput(w, "saving access token faild", 400, nil)
			} else {
				//"thingList":[],"total":1 我列不出来暂不自动处理
				//deviceIDs := listDevices()
				html := `手动输入设备ID（在易微联APP或小程序里查看）：
<form action="?install=2" method="post" name="form1">
	Device ID: <input name="deviceID" type="text">
	<button >submit</button>
<form>
只处理1个设备，多个设备以后再说
`
				htmlOutput(w, html, 200, nil)
			}
		}
	} else {
		if query.Get("install") == "2" {
			data := r.Form
			// 只处理1个设备，多个设备以后再说
			err := saveConfig(1, "deviceIDs", data.Get("deviceID"))
			if err != nil {
				html := "Something error in saving: " + fmt.Sprint(err)
				htmlOutput(w, html, 400, nil)
			} else {
				if err != nil {
					html := "Something error in saving: " + fmt.Sprint(err)
					htmlOutput(w, html, 400, nil)
				} else {
					htmlOutput(w, "Success", 200, nil)
					quit<-1
				}
			}
		} else {
			if query.Get("install") == "1" {
				data := r.Form
				saveConfig(1, "appID", data.Get("appID"))
				saveConfig(1, "appSecret", data.Get("appSecret"))
				err := saveConfig(1, "appRedirect", data.Get("redirectUrl"))
				if err != nil {
					html := "Something error in saving: " + fmt.Sprint(err)
					htmlOutput(w, html, 400, nil)
				} else {
					appInit()
					time1 := time.Now().Unix() * 1000
					signstr := ComputeHmac256(oauthapp.appID + "_" + fmt.Sprint(time1), oauthapp.appSecret)
					url := "https://c2ccdn.coolkit.cc/oauth/index.html" +
					"?clientId=" + oauthapp.appID +
					"&authorization=" + signstr +
					"&seq=" + fmt.Sprint(time1) +
					"&redirectUrl=" + oauthapp.redirectUrl +
					"&nonce=ysun1234" +
					"&grantType=authorization_code" +
					"&state="
					html := `redirecting
<script>
	url = location.href;
	url = url.substr(0, url.indexOf("?"));
	url = "` + url + `".concat(url);
	location.href = url;
</script>`
					//fmt.Fprint(w, html)
					conlog("  redirecting to Ewelink\n")
					htmlOutput(w, html, 200, nil)
				}
			} else {
				html := `
1. 在 https://dev.ewelink.cc/#/console 登录，申请成为开发者（可能要等几天）<br>
2. 新建一个应用（个人开发者只能创建一个），将跳转地址设为下面 Redirect URL 中的url（其实就是当前页面）<br>
3. 将 APPID 与 APP SECRET 填入下方，点击按钮提交给程序
<form action="?install=1" method="post" name="form1">
	App ID: <input name="appID" type="text"><br>
	App Secret: <input name="appSecret" type="password"><br>
	Redirect URL: <input name="redirectUrl" type="text"><br>
	<button >submit</button>
<form>
<script>
	document.form1.redirectUrl.value = location.href;
</script>
	`
				htmlOutput(w, html, 201, nil)
			}
		}
	}
}

func turnLight(deviceID string, turn string) {
	err := setDeviceStatus(1, deviceID, turn)
	if err != nil {
		conlog("  failed! " + fmt.Sprint(err) + "\n")
	} else {
		conlog("  success!\n")
	}
}
func listDevices() (string, error) {
	head := make(map[string]string)
	head["X-CK-Appid"] = oauthapp.appID
	head["Host"] = ewelinkapi.host
	head["Content-Type"] = "application/json"
	head["Authorization"] = "Bearer " + accessToken
	url := ewelinkapi.scheme + ewelinkapi.host + "/v2/device/thing"
	res, err := curl("GET", url, "", head, false, false)
	if err != nil {
		return "", err
	} else {
		body := res.Body
		//fmt.Println(body)
		err1 := readValueInString(string(body), "error")
		if err1 != "0" {
			conlog("  list device faild.\n")
			return "", errors.New(body)
		} else {
			return body, nil
		}
	}
}
func getDeviceStatus(id int, deviceID string) (string, error) {
	head := make(map[string]string)
	head["X-CK-Appid"] = oauthapp.appID
	head["Host"] = ewelinkapi.host
	head["Content-Type"] = "application/json"
	head["Authorization"] = "Bearer " + accessToken
	url := ewelinkapi.scheme + ewelinkapi.host + "/v2/device/thing/status" + "?type=1&id=" + deviceID + "&params=switch"
	res, err := curl("GET", url, "", head, false, false)
	if err != nil {
		return "", err
	} else {
		body := res.Body
		//fmt.Println(body)
		err1 := readValueInString(string(body), "error")
		if err1 != "0" {
			conlog("  get " + deviceID + " status faild.\n")
			return "", errors.New(body)
		} else {
			status := readValueInString(string(body), "switch")
			return status, nil
		}
	}
}
func setDeviceStatus(id int, deviceID string, status string) error {
	head := make(map[string]string)
	head["X-CK-Appid"] = oauthapp.appID
	head["Host"] = ewelinkapi.host
	head["Content-Type"] = "application/json"
	head["Authorization"] = "Bearer " + accessToken
	url := ewelinkapi.scheme + ewelinkapi.host + "/v2/device/thing/status"
	data := "{\"type\":1,\"id\":\"" + deviceID + "\",\"params\":{\"switch\":\"" + status + "\"}}"
	res, err := curl("POST", url, data, head, false, false)
	if err != nil {
		return err
	} else {
		body := res.Body
		//fmt.Println(body)
		err1 := readValueInString(string(body), "error")
		if err1 != "0" {
			conlog("  set " + deviceID + " status faild.\n")
			return errors.New(body)
		} else {
			//status := readValueInString(string(body), "switch")
			return nil
		}
	}
}
func RefreshToken(refreshToken string) error {
	data := "{\"rt\":\"" + refreshToken + "\"}"
	head := make(map[string]string)
	head["X-CK-Appid"] = oauthapp.appID
	head["Host"] = ewelinkapi.host
	head["Content-Type"] = "application/json"
	head["Authorization"] = "Bearer " + accessToken

	res, err := curl("POST", ewelinkapi.scheme + ewelinkapi.host + ewelinkapi.refreshToken, data, head, false, false)
	if err != nil {
		return err
	} else {
		//res.StatusCode
		body := res.Body
		//fmt.Println(body)
		err1 := readValueInString(string(body), "error")
		if err1 != "0" {
			conlog("  Refresh token faild.\n")
			return errors.New(body)
		} else {
			conlog("  saving access token\n")
			accessToken = readValueInString(string(body), "at")
			atet := (time.Now().Unix() +  30 * 24 * 60 * 60) * 1000
			rt := readValueInString(string(body), "rt")
			rtet := (time.Now().Unix() +  60 * 24 * 60 * 60) * 1000
			saveConfig(1, "accessToken", accessToken)
			saveConfig(1, "atExpiredTime", string(atet))
			saveConfig(1, "refreshToken", rt)
			return saveConfig(1, "rtExpiredTime", string(rtet))
		}
	}
}
	//content, err := ioutil.ReadFile(dbFilePath)
	//return ioutil.WriteFile(dbFilePath, []byte(str), 0666)
func sqlite(str string) (string, error) {
	//fmt.Println(str)
	result := ""
	cmd := exec.Command("sqlite3", dbFilePath, str)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "p", err
	}
	if err = cmd.Start(); err != nil {
		return "s", err
	}
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		result += scanner.Text()
	}
	if err = cmd.Wait(); err != nil {
		return "w", err
	}
	return result, nil
}
func saveConfig(id int, key string, value string) error {
	_, err := sqlite("update data set " + key + "=\"" + value + "\" where id=" + strconv.Itoa(id) + ";")
	return err
}
func readConfig(id int, key string) (string, error) {
	return sqlite("select " + key + " from data where id=" + strconv.Itoa(id) + ";" )
}
/*
admin (id user pass)
data (id appID appSecret appRedirect accessToken atExpiredTime refreshToken rtExpiredTime deviceIDs )
*/
func validToken() bool {
	_, err := os.Stat(dbFilePath)
	if err != nil {
		sqlite("create table admin (id integer primary key, user char(20), pass char(20));")
		sqlite("create table data (id integer primary key, appID text, appSecret text, appRedirect text, accessToken text, atExpiredTime text, refreshToken text, rtExpiredTime text, deviceIDs text);")
		sqlite("insert into data (id) values (1);")
		return false
	} else {
		accessToken, err = readConfig(1, "accessToken")
		if err != nil {
			//fmt.Println("_" + accessToken + "_")
			fmt.Println(err)
			return false
		} else {
			if accessToken == "" {
				return false
			}
			appInit()
			//fmt.Println("_" + accessToken + "_")
			atExpiredTime, err := readConfig(1, "atExpiredTime")
			if err != nil {
				fmt.Println(err)
				return false
			}
			if len(atExpiredTime) > 3 {
				atExpiredTime = atExpiredTime[0:len(atExpiredTime)-3]
			}
			time1, err := strconv.Atoi(atExpiredTime)
			if err != nil {
				fmt.Println(err)
			}
			time2 := int(time.Now().Unix() + 15*24*60*60)
			if time1 < time2 {
				refreshToken, err := readConfig(1, "refreshToken")
				if err != nil {
					fmt.Println(err)
				}
				err = RefreshToken(refreshToken)
				if err != nil {
					conlog(alertlog(fmt.Sprint(err)) + "\n")
					return false
				}
			}
			return true
		}
		return true
	}
}
func removeStrbefor(text string, pre string) string {
	for strings.Index(text, pre) > -1 {
		text = text[(strings.Index(text, pre) + 1):]
	}
	return text
}
func readValueInString(text string, key string) string {
	for strings.Index(text, key) > -1 {
		key1 := text[strings.Index(text, key):]
		key1 = key1[0:strings.Index(key1, "\"")]
		if key1 == key {
			value := text[(strings.Index(text, key) + len(key) + 1):]
			if strings.Index(value, ",") > -1 {
				if strings.Index(value, ",") < strings.Index(value, "\"") {
					value = value[0:strings.Index(value, ",")]
					value = removeStrbefor(value, " ")
					value = removeStrbefor(value, ":")
				} else {
					value = value[(strings.Index(value, "\"") + 1):]
					value = value[0:strings.Index(value, "\"")]
				}
			} else {
				if strings.Index(value, "\"") > -1 {
					value = value[(strings.Index(value, "\"") + 1):]
					value = value[0:strings.Index(value, "\"")]
				} else {
					value = value[0:strings.Index(value, "}")]
					if strings.Index(value, "\n") > -1 {
						value = value[0:strings.Index(value, "\n")]
					}
					value = removeStrbefor(value, " ")
					value = removeStrbefor(value, ":")
				}
			}
			return value
		}
		text = text[(strings.Index(text, key) + len(key)):]
	}
	return ""
}
func ComputeHmac256(message string, secret string) string {
    key := []byte(secret)
    h := hmac.New(sha256.New, key)
    h.Write([]byte(message))
    return base64.StdEncoding.EncodeToString(h.Sum(nil))
}
func htmlOutput(w http.ResponseWriter, body string, code int, head map[string]string) {
	w.Header().Set("Content-Type", "text/html")
	for k, v := range head {
		w.Header().Set(k, v)
	}
	w.WriteHeader(code)
	body = `
<meta http-equiv="Content-Type" content="text/html; charset=UTF-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
` + body
	w.Write([]byte(body))
}

type HttpResult struct {
	StatusCode int
	Header http.Header
	Body string
}
func curl(method string, url string, data string, header map[string]string, returnHeader bool, location bool) (HttpResult, error) {
	var result HttpResult
	var err error
	//fmt.Println("初始", result.StatusCode)
	if len(url) < 7 || ( url[0:7] != "http://" && url[0:8] != "https://" ) {
		url = "http://" + url
	}
	var req *http.Request
	req, err = http.NewRequest(method, url, strings.NewReader(data))
	if err != nil {
		fmt.Println(err)
	} else {
		//fmt.Println(res)
		for k, v := range header {
			req.Header.Add(k, v)
		}
		client := &http.Client{}
		var res *http.Response
		res, err = client.Do(req)
		if err != nil {
			fmt.Println(err)
		} else {
			//fmt.Println(res.StatusCode)
			//fmt.Println(res.Header)
			//fmt.Println(res.Body)
			result.StatusCode = res.StatusCode
			result.Header = res.Header
			var body []byte
			body, err = ioutil.ReadAll(res.Body)
			if err != nil {
				fmt.Println(err)
			} else {
				//fmt.Println(string(body))
				result.Body = string(body)
			}
		}
	}
	return result, err
}
