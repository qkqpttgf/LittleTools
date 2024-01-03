package main

import (
	"bufio"
	"crypto/hmac"
	"crypto/md5"
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
	"runtime"
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
func appInit(id int) {
	oauthapp.appID, _ = readConfig(id, "appID")
	oauthapp.appSecret, _ = readConfig(id, "appSecret")
	oauthapp.redirectUrl, _ = readConfig(id, "appRedirect")
}

var quit chan int
var accessToken string
var slash string
var isCmdWindow bool

func main() {
	conlog(passlog("Program Start") + "\n")
	defer conlog(passlog("Program End") + "\n")
	isCmdWindow = false
	if runtime.GOOS == "windows" {
		slash = "\\"
		cmdVersion, _ := getCmdVersion()
		//fmt.Println(cmdVersion)
		if cmdVersion < 10 {
			isCmdWindow = true
		}
	} else {
		slash = "/"
	}
	parseCommandLine()
	apiInit()
	if validToken() {
		// token 有效
		if operateLight != "" {
			err := turnLight(targetLight, operateLight)
			if err != nil {
				conlog("  failed! " + fmt.Sprint(err) + "\n")
			} else {
				conlog("  success!\n")
			}
		} else{
			if startWeb {
				startSrv()
				stopSrv()
			} else {
				//a,_ := listDevices()
				//fmt.Println(a)
				dIDs, _ := readConfig(1, "deviceIDs")
				deviceIDs := strings.Split(dIDs, ",")
				for _, device := range deviceIDs {
					status, _ := getDeviceStatus(1, device)
					conlog("  " + device + ": " + status + "\n")
				}
				useage()
			}
		}
	} else {
		useage()
		conlog(alertlog("no valid token, OAuth start.") + "\n")
		startTmpSrv()
		stopSrv()
	}
	/*fmt.Printf("Press any key to exit...\n")
	exec.Command("stty","-F","/dev/tty","cbreak","min","1").Run()
	exec.Command("stty","-F","/dev/tty","-echo").Run()
	defer exec.Command("stty","-F","/dev/tty","echo").Run()
	b := make([]byte, 1)
	os.Stdin.Read(b)*/
	//fmt.Println(a,b)
}

func parseCommandLine() {
	configFile := false
	turnLight1 := false
	softPath := ""
	conlog("Commands:\n")
	for argc, argv := range os.Args {
		fmt.Printf("  %d: %v\n", argc, argv)
		if argc == 0 {
			softPath = argv
			pos := strings.LastIndex(softPath, slash)
			if pos > -1 {
				softPath = softPath[0:pos+1]
			} else {
				softPath = ""
			}
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
		dbFilePath = softPath + "EweConfig.db"
	}
	conlog("Using datebase:\n  " + warnlog(dbFilePath) + "\n")
}
func useage() {
	html := `Useage:
  -c|-config databaseFile  set db
  web                      start a web page
  turnon deviceID          turn on the device
  turnoff deviceID         turn off the device
`
	//fmt.Print(html)
	conlog(html)
}

func conlog(log string) {
	layout := "[2006-01-02 15:04:05.000] "
	strTime := (time.Now()).Format(layout)
	fmt.Print("\r", strTime, log)
}
func alertlog(log string) string {
	if isCmdWindow {
		return log
	} else {
		return fmt.Sprintf("\033[91;5m%s\033[0m", log)
	}
}
func warnlog(log string) string {
	if isCmdWindow {
		return log
	} else {
		return fmt.Sprintf("\033[92;93m%s\033[0m", log)
	}
}
func passlog(log string) string {
	if isCmdWindow {
		return log
	} else {
		return fmt.Sprintf("\033[92;32m%s\033[0m", log)
	}
}

func startSrv() {
	http.HandleFunc("/", route)
	Server = listenHttp("", 60575)
	if Server == nil {
		conlog("Server start failed\n")
	} else {
		conlog("Server started\n")
		//waitSYS()
		waitWeb()
	}
}
func startTmpSrv() {
	http.HandleFunc("/", oauthroute)
	Server = listenHttp("", 60576)
	if Server == nil {
		conlog("OAuth Server start failed\n")
	} else {
		conlog("OAuth Server started\n")
		conlog("Please visit http://{YourIP}:60576/ in a browser.\n")
		fmt.Print("Waiting...")
		waitOauth()
	}
}
func listenHttp(bindIP string, port int) *http.Server {
	conlog(fmt.Sprintf("starting http at %v:%d\n", bindIP, port))
	srv := &http.Server{Addr: fmt.Sprintf("%v:%d", bindIP, port), Handler: nil}
	srv1, err := net.Listen("tcp", fmt.Sprintf("%v:%d", bindIP, port))
	if err != nil {
		conlog(fmt.Sprint(err, "\n"))
		conlog(alertlog("http start failed") + "\n")
		return nil
	}
	go srv.Serve(srv1)
	return srv
}
func stopSrv() {
	if Server != nil {
		err := Server.Close()
		if err != nil {
			conlog(fmt.Sprint(err, "\n"))
		} else {
			conlog("  http closed\n")
		}
	}
}
func waitSYS() {
	sysSignalQuit := make(chan os.Signal, 1)
	defer close(sysSignalQuit)
	signal.Notify(sysSignalQuit, syscall.SIGINT, syscall.SIGTERM)
	<- sysSignalQuit
	fmt.Print("\n")
}
func waitWeb() {
	count := 0
	quit = make(chan int, 2)
	defer close(quit)
	maxcount := 5
	go func() {
		waitSYS()
		quit <- -1
	}()
	for count > -1 {
		if count == -1 {
			conlog("Program exiting.\n")
		} else {
			if count > 1 {
				fmt.Print(".")
				if count > maxcount {
					count = 0
					//fmt.Print("count")
				}
			} else {
				fmt.Print("\r")
				str := "Waiting visitor"
				for count1 := count; count1 < maxcount + len(str) + 1; count1++ {
					fmt.Print(" ")
				}
				fmt.Print("\r")
				if count == 1 {
					fmt.Print(str)
				}
			}
			go func(c int) {
				time.Sleep(1 * time.Second)
				if c == count {
					quit <- count + 1
				}
			}(count)
			count = <- quit
		}
	}
}
func waitOauth() {
	count := 1
	quit = make(chan int, 2)
	defer close(quit)
	go func() {
		waitSYS()
		quit <- 0
	}()
	for count > 0 {
		if count == 0 {
			conlog("Program exiting.\n")
		} else {
			if count > 15 {
				fmt.Print("\n")
				conlog("Timeout, OAuth exit.\n")
				count = -1
			}
		}
		go func() {
			time.Sleep(60 * time.Second)
			quit <- count + 1
		}()
		count = <- quit
	}
}

func route(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	quit <- 0
	r.ParseForm()
	//r.TLS != nil
	conlog(warnlog(fmt.Sprintln(r.Method, r.URL)))
	//conlog(warnlog(fmt.Sprintln(r.Method, r.Host, r.URL, r.Header.Get("X-Real-IP"), r.RemoteAddr)))
	fmt.Println(r)
	//fmt.Println(r.Header)
	path := r.URL.Path
	//fmt.Println("_" + path)
//    query := r.URL.Query()
    //fmt.Println(query.Get("a"))
	data := r.Form
	if r.Method == "POST" {
		fmt.Println(data)
	}

	if path != "/" {
		return
	}
	html := ""
	admincookie1, err := r.Cookie("admin")
	admincookie := ""
	if err != nil {
		admincookie = ""
	} else {
		admincookie = admincookie1.Value
	}
	if admincookie != "" {
		if data.Get("device") != "" && (data.Get("action") == "on" || data.Get("action") == "off") {
			httpCode := 200
			err = turnLight(data.Get("device"), data.Get("action"))
			if err != nil {
				html += "  failed! " + fmt.Sprint(err) + "\n"
				httpCode = 400
			} else {
				html += "  success!\n"
			}
			html += `<meta http-equiv="refresh" content="3;URL=">`
			htmlOutput(w, html, httpCode, nil)
		} else {
			dIDs, _ := readConfig(1, "deviceIDs")
			deviceIDs := strings.Split(dIDs, ",")
			for _, device := range deviceIDs {
				status, _ := getDeviceStatus(1, device)
				//conlog("  " + device + ": " + status + "\n")
				html += `
<form name="` + device + `Form" method="post">
	<input name="device" value="` + device + `" size="10" disabled>状态: ` + status + `
	<button name="action" value="on"`
				if status == "on" {
					html += " disabled"
				}
				html += `>开</button>
	<button name="action" value="off"`
				if status == "off" {
					html += " disabled"
				}
				html += `>关</button>
</form>`
			}
			htmlOutput(w, html, 200, nil)
		}
	} else {
		if data.Get("user") != "" && data.Get("pass") != "" {
			adminuser, _ := readConfig(1, "user")
			adminpass, _ := readConfig(1, "pass")
			if data.Get("user") == adminuser && data.Get("pass") == adminpass {
				t := int(time.Now().Unix() + 24 * 60 * 60)
				md5 := passHashCookie(data.Get("user"), data.Get("pass"), t)
				html = `
<meta http-equiv="refresh" content="3;URL=` + path + `">
Success
<script>
	var expd = new Date();
	expd.setTime(` + fmt.Sprint(t) + `000);
	var expires = "expires=" + expd.toGMTString();
	document.cookie="admin=` + data.Get("user") + `:` + md5 + `@` + fmt.Sprint(t) + `; " + expires;
</script>
`
				htmlOutput(w, html, 200, nil)
			} else {
				html = `<meta http-equiv="refresh" content="3;URL=` + path + `">Failed`
				htmlOutput(w, html, 403, nil)
			}
		} else {
			html = `登录
<form action="" method="post" name="form1">
	username: <input name="user" type="text"><br>
	password: <input name="pass" type="password"><br>
	<button>提交</button>
<form>`
			htmlOutput(w, html, 401, nil)
		}
	}

}
func checkCookie(admin string) bool {
	pos1 := strings.Index(admin, ":")
	if pos1 < 0 {
		return false
	}
	pos2 := strings.Index(admin, "@")
	if pos2 < 0 {
		return false
	}
	user := admin[0:pos1]
	adminuser, _ := readConfig(1, "user")
	if user != adminuser {
		return false
	}
	md5 := admin[pos1+1:pos2]
	t, _ := strconv.Atoi(admin[pos2+1:])
	nt := int(time.Now().Unix())
	if t < nt {
		return false
	}
	adminpass, _ := readConfig(1, "pass")
	if passHashCookie(user, adminpass, t) == md5 {
		return true
	} else {
		return false
	}
}
func passHashCookie(u string, p string, t int) string {
	return fmt.Sprintf("%x", md5.Sum([]byte(u + "@" + p + "(" + fmt.Sprint(t))))
}
func oauthroute(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	r.ParseForm()
	//fmt.Println(r.TLS != nil, r.Host, r.URL)
	//fmt.Println(r.Header)
	path := r.URL.Path
	//fmt.Println("_" + path)
    query := r.URL.Query()
    //fmt.Println(query.Get("a"))
	data := r.Form
	//fmt.Println(data)
	//-------------------------
	if path != "/" {
		return
	}
	if query.Get("code") != "" {
		// 有code
		conlog("  received a code\n")
		data1 := "{\"code\":\"" + query.Get("code") + "\", \"redirectUrl\":\"" + oauthapp.redirectUrl + "\", \"grantType\":\"authorization_code\"}"
		head := make(map[string]string)
		head["X-CK-Appid"] = oauthapp.appID
		head["Content-Type"] = "application/json"
		head["Authorization"] = "Sign " + ComputeHmac256(data1, oauthapp.appSecret)
		head["Host"] = ewelinkapi.host

		res, err := curl("POST", ewelinkapi.scheme + ewelinkapi.host + ewelinkapi.oauthToken, data1, head)
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
				conlog("  saving access token failed\n")
				htmlOutput(w, "saving access token failed", 400, nil)
			} else {
				//deviceIDs := listDevices()
				//"thingList":[],"total":1 我列不出来暂不会自动处理
				html := `手动输入设备ID（在易微联APP或小程序里查看）：
<form action="?install=2" method="post" name="form1">
	Device ID: <input name="deviceID" type="text"><br>
	<button>提交</button>
<form>
只处理1个设备，多个设备以后再说
`
				htmlOutput(w, html, 200, nil)
			}
		}
	} else {
		if query.Get("install") == "2" {
			// 只处理1个设备，多个设备以后再说
			err := saveConfig(1, "deviceIDs", data.Get("deviceID"))
			if err != nil {
				html := "Something error in saving: " + fmt.Sprint(err)
				htmlOutput(w, html, 400, nil)
			} else {
				htmlOutput(w, "Success", 200, nil)
				conlog("  Success\n")
				quit <- 0
			}
		} else {
			if query.Get("install") == "1" {
				saveConfig(1, "appID", data.Get("appID"))
				saveConfig(1, "appSecret", data.Get("appSecret"))
				err := saveConfig(1, "appRedirect", data.Get("redirectUrl"))
				if err != nil {
					html := "Something error in saving: " + fmt.Sprint(err)
					htmlOutput(w, html, 400, nil)
				} else {
					conlog("  redirecting to Ewelink\n")
					appInit(1)
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
					htmlOutput(w, html, 200, nil)
				}
			} else {
				fmt.Print("\r")
				pass, _ := readConfig(1, "pass")
				//fmt.Println(pass)
				if pass != "" {
					conlog("  OAuth Page Start\n")
					html := `
1. 在 https://dev.ewelink.cc/#/console 登录，申请成为开发者（可能要等几天）<br>
2. 新建一个应用（个人开发者只能创建一个），将跳转地址设为下面 Redirect URL 中的url（其实就是当前页面）<br>
3. 将 APPID 与 APP SECRET 填入下方，点击按钮提交给程序
<form action="?install=1" method="post" name="form1">
	App ID: <input name="appID" type="text"><br>
	App Secret: <input name="appSecret" type="password"><br>
	Redirect URL: <input name="redirectUrl" type="text"><br>
	<button>提交</button>
<form>
<script>
	document.form1.redirectUrl.value = location.href;
</script>
`
					htmlOutput(w, html, 201, nil)
				} else {
					if data.Get("user") != "" && data.Get("pass") != "" {
						saveConfig(1, "user", data.Get("user"))
						err := saveConfig(1, "pass", data.Get("pass"))
						if err != nil {
							conlog("  Set admin failed\n")
							html := `failed
<meta http-equiv="refresh" content="5;URL=">
`
							htmlOutput(w, html, 400, nil)
						} else {
							conlog("  Setting admin\n")
							html := `Success
<meta http-equiv="refresh" content="3;URL=">
`
							htmlOutput(w, html, 201, nil)
						}
					} else {
						conlog("  Please set admin first\n")
						html := `
Set admin user:
<form action="" method="post" name="form1">
	username: <input name="user" type="text"><br>
	password: <input name="pass" type="password"><br>
	<button>提交</button>
<form>
`
						htmlOutput(w, html, 201, nil)
					}
				}
			}
		}
	}
}

func turnLight(deviceID string, turn string) error {
	return setDeviceStatus(1, deviceID, turn)
}
func listDevices() (string, error) {
	head := make(map[string]string)
	head["X-CK-Appid"] = oauthapp.appID
	head["Host"] = ewelinkapi.host
	head["Content-Type"] = "application/json"
	head["Authorization"] = "Bearer " + accessToken
	url := ewelinkapi.scheme + ewelinkapi.host + "/v2/device/thing"
	res, err := curl("GET", url, "", head)
	if err != nil {
		return "", err
	} else {
		body := res.Body
		//fmt.Println(body)
		err1 := readValueInString(string(body), "error")
		if err1 != "0" {
			conlog("  list device failed.\n")
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
	res, err := curl("GET", url, "", head)
	if err != nil {
		return "", err
	} else {
		body := res.Body
		//fmt.Println(body)
		err1 := readValueInString(string(body), "error")
		if err1 != "0" {
			conlog("  get " + deviceID + " status failed.\n")
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
	res, err := curl("POST", url, data, head)
	if err != nil {
		return err
	} else {
		body := res.Body
		//fmt.Println(body)
		err1 := readValueInString(string(body), "error")
		if err1 != "0" {
			conlog("  set " + deviceID + " status failed.\n")
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

	res, err := curl("POST", ewelinkapi.scheme + ewelinkapi.host + ewelinkapi.refreshToken, data, head)
	if err != nil {
		return err
	} else {
		//res.StatusCode
		body := res.Body
		//fmt.Println(body)
		err1 := readValueInString(string(body), "error")
		if err1 != "0" {
			conlog("  Refresh token failed.\n")
			return errors.New(body)
		} else {
			conlog("  saving access token\n")
			accessToken = readValueInString(string(body), "at")
			atet := (time.Now().Unix() +  30 * 24 * 60 * 60) * 1000
			rt := readValueInString(string(body), "rt")
			rtet := (time.Now().Unix() +  60 * 24 * 60 * 60) * 1000
			saveConfig(1, "accessToken", accessToken)
			saveConfig(1, "atExpiredTime", strconv.FormatInt(atet, 10))
			saveConfig(1, "refreshToken", rt)
			return saveConfig(1, "rtExpiredTime", strconv.FormatInt(rtet, 10))
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
	//fmt.Println(result, str)
	return result, nil
}
func saveConfig(id int, key string, value string) error {
	table := "data"
	if key == "user" || key == "pass" {
		table = "admin"
	}
	_, err := sqlite("update " + table + " set " + key + "=\"" + value + "\" where id=" + strconv.Itoa(id) + ";")
	return err
}
func readConfig(id int, key string) (string, error) {
	table := "data"
	if key == "user" || key == "pass" {
		table = "admin"
	}
	return sqlite("select " + key + " from " + table + " where id=" + strconv.Itoa(id) + ";" )
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
		sqlite("insert into admin (id) values (1);")
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
			dIDs, _ := readConfig(1, "deviceIDs")
			if dIDs == "" {
				return false
			}
			appInit(1)
			//fmt.Println("_" + accessToken + "_")
			atExpiredTime, _ := readConfig(1, "atExpiredTime")
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
			_, err = listDevices()
			if err != nil {
				return false
			} else {
				return true
			}
		}
	}
}
func removeStrbefor(text string, pre string) string {
	for strings.Index(text, pre) > -1 {
		text = text[(strings.Index(text, pre) + 1):]
	}
	return text
}
func readValueInString(text string, key string) string {
	key = "\"" + key + "\""
	if strings.Index(text, key) > -1 {
		value := text[(strings.Index(text, key) + len(key)):]
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
func getCmdVersion() (int, error) {
	//fmt.Println(str)
	result := ""
	version := 0
	cmd := exec.Command("cmd.exe", "-v")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return -1, err
	}
	if err = cmd.Start(); err != nil {
		return -2, err
	}
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		result += scanner.Text()
	}
	if err = cmd.Wait(); err != nil {
		return -3, err
	}
	//fmt.Println(result, str)
	if result != "" {
		result = result[strings.Index(result, "["):]
		result = result[strings.Index(result, " ")+1:strings.Index(result, ".")]
		version, err = strconv.Atoi(result)
		if err != nil {
			return -5, err
		} else {
			return version, nil
		}
	} else {
		return -4, errors.New("Empty")
	}
}

type HttpResult struct {
	StatusCode int
	Header http.Header
	Body string
}
func curl(method string, url string, data string, header map[string]string) (HttpResult, error) {
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
