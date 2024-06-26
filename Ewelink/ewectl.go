package main

import (
	"bufio"
	"context"
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

// 用于处理命令行是什么操作
var dbFilePath string
var operateLight string
var targetLight string
var startWeb bool
var startOauth bool

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
	ewelinkapi.host = "cn-apia.coolkit.cn" // 中国区
	ewelinkapi.oauthToken = "/v2/user/oauth/token"
	ewelinkapi.refreshToken = "/v2/user/refresh"
	ewelinkapi.viewStatus = "/v2/device/thing/status"
}
type OauthApp struct {
	appID string
	appSecret string
	redirectUrl string
	accessToken string
}
var oauthapp OauthApp
// 读取token与client
func tokenInit(id int) bool {
	oauthapp.accessToken, _ = readConfig("token", "accessToken", id)
	id_client_string, _ := readConfig("token", "clientID", id)
	id_client, _ := strconv.Atoi(id_client_string)
	//fmt.Println("client id ", id_client)
	appInit(id_client)
	return validToken(id)
}
func appInit(id int) {
	oauthapp.appID, _ = readConfig("client", "appID", id)
	oauthapp.appSecret, _ = readConfig("client", "appSecret", id)
	oauthapp.redirectUrl, _ = readConfig("client", "appRedirect", id)
}
type DeviceStatus struct {
	name string
	deviceID string
	online bool
	switches []string
}

var quit chan int
var slash string
var isCmdWindow bool

func main() {
	conlog(passlog("Program Start") + "\n")
	defer conlog(warnlog("Program End") + "\n")
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
	if !parseCommandLine() {
		// 命令行不是期望的格式
		conlog(alertlog("command error.") + "\n")
		useage()
		return
	}
	if !checkDatabase() {
		// 数据库不对
		return
	}
	apiInit()
	pass, _ := readConfig("admin", "pass", 0)
	if pass == "" {
		// 初始化设置
		conlog("Please set config " + warnlog("on a web page") + " in a browser.\n")
		startTmpSrv()
		return
	}
	if operateLight != "" {
		// 开关操作
		conlog(" Turnning " + operateLight + " " + targetLight + "\n")
		err := turnLight(targetLight, operateLight)
		if err != nil {
			conlog("  failed!\n  " + fmt.Sprint(err) + "\n")
		} else {
			conlog("  success!\n")
		}
		return
	}
	if startWeb {
		// 开启一个web页面来操作
		startSrv()
		return
	}
	if startOauth {
		// 获取token
		startTmpSrv()
		return
	}

	// 没有指定操作，显示各设备状态
	useage()
	dIDs, _ := readConfig("device", "id,deviceID", 0)
	//fmt.Println(dIDs)
	if dIDs != "" {
		deviceIDs := strSplitLine(dIDs)
		for _, device1 := range deviceIDs {
			id := device1[0:strings.Index(device1, "|")]
			device := device1[strings.Index(device1, "|")+1:]
			//fmt.Println(device)
			deviceStatus, err := checkDeviceOnline(device)
			tmp := ""
			if err != nil {
				tmp = fmt.Sprint(err)
			} else {
				if deviceStatus.online {
					if len(deviceStatus.switches) > 1 {
						for i, s := range deviceStatus.switches {
							tmp += "chanel_" + fmt.Sprint(i) + " " + s + ", "
						}
						tmp = tmp[0:len(tmp)-2]
					} else {
						tmp = deviceStatus.switches[0]
					}
				} else {
					tmp = "offline"
				}
			}
			conlog(id + ", " + device + ", " + deviceStatus.name + ": " + tmp + "\n")
		}
	} else {
		conlog(alertlog("No devices, OAuth start.") + "\n")
		startTmpSrv()
	}
	return
	/*fmt.Printf("Press any key to exit...\n")
	exec.Command("stty","-F","/dev/tty","cbreak","min","1").Run()
	exec.Command("stty","-F","/dev/tty","-echo").Run()
	defer exec.Command("stty","-F","/dev/tty","echo").Run()
	b := make([]byte, 1)
	os.Stdin.Read(b)*/
	//fmt.Println(a,b)
}

func parseCommandLine() bool {
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
			continue
		}
		if argv == "web" {
			startWeb = true
			continue
		}
		if argv == "add" {
			startOauth = true
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

		// not 
		conlog("Unknown parameter: " + argv + "\n")
		return false
	}
	if operateLight != "" && operateLight != "on" && operateLight != "off" {
		conlog(alertlog("invalid operate: \"" + operateLight + "\"\n"))
		return false
	}
	if operateLight != "" && targetLight == "" {
		conlog(alertlog("turn empty light\n"))
		return false
	}
	todoNum := 0
	if operateLight != "" {
		todoNum++
	}
	if startWeb {
		todoNum++
	}
	if startOauth {
		todoNum++
	}
	if todoNum > 1 {
		conlog(alertlog("please do only one thing in one time\n"))
		return false
	}
	if dbFilePath == "" {
		dbFilePath = softPath + "EwelinkConfig.db"
	}
	conlog("Using datebase:\n  " + warnlog(dbFilePath) + "\n")
	return true
}
func useage() {
	html := `Useage:
  -c|-config PathOfDBfile   set path of database
  add                       start a web page to add device
  web                       start a web page to manage
  turnon ID[:num]           turn on the device
  turnoff ID[:num]          turn off the device

(the ID is device id string or serial number in datebase
for multi-outlets device, you can use "ID:number" to appoint a channel)

`
	//fmt.Print(html)
	conlog(html)
}
func checkDatabase() bool {
	createTableSQL := `CREATE TABLE admin (id integer primary key, user char(20), pass char(20));
CREATE TABLE client (id integer primary key, appID text, appSecret text, appRedirect text);
CREATE TABLE device (id integer primary key, deviceID text, tokenID text);
CREATE TABLE token (id integer primary key, clientID text, accessToken text, atExpiredTime text, refreshToken text, rtExpiredTime text);`
	_, err := os.Stat(dbFilePath)
	if err != nil {
		conlog("Database file " + alertlog("not found") + ", it will " + warnlog("be created") + ".\n")
		sqlArr := strSplitLine(createTableSQL)
		for _, sql := range sqlArr {
			fmt.Println("  " + sql)
			_, err = sqlite(sql)
			if err != nil {
				conlog(alertlog("Create fail\n") + sql + "\n")
				return false
			}
		}
		conlog(passlog("Create done.\n"))
		return true
	} else {
		conlog("Checking database.\n")
		create_Arr := strSplitLine(createTableSQL)
		databaseOK := false
		existSql, _ := sqlite(".schema")
		exist_Arr := strSplitLine(existSql)
		exist_Map := make(map[string]string)
		for _, sql := range exist_Arr {
			if sql != "" {
				exist_Map[sql] = "1"
			}
		}
		for _, sql := range create_Arr {
			delete(exist_Map, sql)
		}
		if len(exist_Map) == 0 {
			databaseOK = true
		}

		if databaseOK {
			conlog(passlog("Database OK\n"))
			return true
		} else {
			conlog(alertlog("Database Error\n") + warnlog("  It may not a database for ewectl, or it is an old version.\n  Please remove it to create a new one.\n"))
			//conlog(sql + "\n")
			return false
		}
	}
}
func strSplitLine(target string) []string {
	return strings.FieldsFunc(target, func(r rune) bool {
        return r == '\r' || r == '\n'
    })
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
		//return fmt.Sprintf("\033[92;93m%s\033[0m", log)
		return fmt.Sprintf("\033[33;33m%s\033[0m", log)
	}
}
func passlog(log string) string {
	if isCmdWindow {
		return log
	} else {
		return fmt.Sprintf("\033[92;32m%s\033[0m", log)
		//return fmt.Sprintf("\033[92;60m%s\033[0m", log)
	}
}

func startSrv() {
	http.HandleFunc("/", route)
	port := 60575
	Server := listenHttp("", port)
	defer stopSrv(Server)
	if Server == nil {
		conlog("Server start failed\n")
	} else {
		conlog("Server started\n")
		showListening(port)
		waitWeb()
	}
}
func startTmpSrv() {
	http.HandleFunc("/", oauthroute)
	port := 60576
	Server := listenHttp("", port)
	defer stopSrv(Server)
	if Server == nil {
		conlog("OAuth Server start failed\n")
	} else {
		conlog("OAuth Server started\n")
		showListening(port)
		waitOauth(10 * 60)
	}
}
func showListening(port int) {
	conlog("Please visit one of url(s) below in a browser.\n")
	for _, ip := range getLocalIPS() {
		if strings.Index(ip, ":") > -1 { // ipv6加上中括号
			ip = "[" + ip + "]"
		}
		fmt.Printf("     http://%s:%d/\n", ip, port)
	}
}
func getLocalIPS() []string {
	var ips []string
	interfaces, err := net.Interfaces()
	if err != nil {
		panic(err)
	}

	for _, inter := range interfaces {
		addrs, err := inter.Addrs()
		if err != nil {
			panic(err)
		}

		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}

			if ipNet.IP.IsLinkLocalUnicast() || ipNet.IP.IsLoopback() {
				continue
			}

			ips = append(ips, ipNet.IP.String())
		}
	}
	//fmt.Println(ips)
	return ips
}
func listenHttp(bindIP string, port int) *http.Server {
	conlog(fmt.Sprintf("Starting http at %v:%d\n", bindIP, port))
	srv := &http.Server{Addr: fmt.Sprintf("%v:%d", bindIP, port), Handler: nil}
	//srv1, err := net.Listen("tcp", fmt.Sprintf("%v:%d", bindIP, port))
	srv1, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		conlog(fmt.Sprint(err, "\n"))
		conlog(alertlog("http start failed") + "\n")
		return nil
	}
	go srv.Serve(srv1)
	return srv
}
func stopSrv(Server *http.Server) {
	if Server != nil {
		// 不强制关闭，会等当前会话结束 // elegant graceful
		err := Server.Shutdown(context.Background())
		if err != nil {
			conlog(fmt.Sprint(err, "\n"))
		}
		err = Server.Close()
		if err != nil {
			conlog(fmt.Sprint(err, "\n"))
		} else {
			conlog("http closed.\n")
		}
	}
}
func waitSYS() {
	// 等候系统中断，比如按ctrl c
	sysSignalQuit := make(chan os.Signal, 1)
	defer close(sysSignalQuit)
	signal.Notify(sysSignalQuit, syscall.SIGINT, syscall.SIGTERM)
	<- sysSignalQuit
	fmt.Print("\n")
}
func waitWeb() {
	quit = make(chan int, 1)
	defer close(quit)
	go func() {
		waitSYS()
		quit <- -1
	}()
	// 底部跑马灯文字
	go displayHorseRaceLamp()
	// 等ctrl c
	<- quit
}
// 等OAuth流程，超时后程序直接结束，避免下次运行还在占用端口
// 每次访问后刷新超时时间
func waitOauth(expireSecond int) {
	defer conlog("OAuth exiting\n")
	count := 0
	quit = make(chan int, 1)
	defer close(quit)
	go func() {
		waitSYS()
		quit <- -1
	}()
	// 底部跑马灯文字
	go displayHorseRaceLamp()
	// 判断超时
	for count > -1 {
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			time.Sleep(time.Second * time.Duration(expireSecond))
			// 超时时间后判断当前ctx是否存活
			select {
				case <- ctx.Done() :
					// ctx被杀掉了，说明有其它地方给quit传值了
					// 本routine结束不做操作
					return
				default :
					// ctx存活，说明其它地方无操作
					// 超时，给通道传值结束程序
					conlog("Timeout\n")
					quit <- -1
			}
		}()
		// 等待网页路由触发传值，或上方Sleep后传值
		count = <- quit
		// 有值传入后，结束本次ctx，开始下一次循环
		cancel();
	}
}
// 跑马灯显示字符，死循环，未考虑结束
func displayHorseRaceLamp() {
	str := "Waiting visitor ..."
	runstr := []rune(str) // 以防有中文字
	//fmt.Println(len(str), len(runstr))
	count := 0
	for count > -1 {
		//fmt.Println(count)
		if count > len(runstr) {
			count = 0
			fmt.Print("\r")
			width := screenWidth()
			if width == 0 {
				width = len(str)
			}
			for i := 0; i < width; i++ {
				fmt.Print(" ")
			}
		}
		fmt.Print("\r" + string(runstr[0:count]))
		time.Sleep(1 * time.Second)
		count++
	}
}
func screenWidth() int {
	cmd := exec.Command("tput", "cols")
	result_b, err := cmd.Output()
	if err == nil {
		result := strings.TrimSpace(string(result_b))
		width, err := strconv.Atoi(result)
		if err == nil {
			return width
		}
	}
	return 0
}

func route(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	//quit <- 0
	r.ParseForm()
	//r.TLS != nil
	conlog(warnlog(fmt.Sprintln(r.Method, r.URL)))
	//conlog(warnlog(fmt.Sprintln(r.Method, r.Host, r.URL, r.Header.Get("X-Real-IP"), r.RemoteAddr)))
	fmt.Println("\r", r)
	//fmt.Println(r.Header)
	path := r.URL.Path
	//fmt.Println("_" + path)
//    query := r.URL.Query()
    //fmt.Println(query.Get("a"))
	data := r.Form
	//if r.Method == "POST" {
	//	fmt.Println(data)
	//}

	if path != "/" {
		return
	}
	html := ""
	if checkAdminShowLoginPage(w, r) {
		if data.Get("device") != "" && (data.Get("action") == "on" || data.Get("action") == "off") {
			httpCode := 200
			device := data.Get("device")
			if data.Get("outlet") != "" {
				device += ":" + data.Get("outlet")
			}
			err := turnLight(device, data.Get("action"))
			if err != nil {
				html += "  failed! " + fmt.Sprint(err) + "\n"
				httpCode = 400
			} else {
				html += "  success!\n"
			}
			html += `<meta http-equiv="refresh" content="3;URL=">`
			htmlOutput(w, html, httpCode, nil)
		} else {
			dIDs, _ := readConfig("device", "id,deviceID", 0)
			//fmt.Println(dIDs)
			deviceIDs := strSplitLine(dIDs)
			for _, device1 := range deviceIDs {
				id := device1[0:strings.Index(device1, "|")]
				device := device1[strings.Index(device1, "|")+1:]
				//fmt.Println(device)
				deviceStatus, err := checkDeviceOnline(device)
				html +=`
` + id + `, <div style="display: inline-block;">
		<input name="devicename" placeholder="设备名称" value="` + deviceStatus.name + `" size="6" readonly><br>
		<input name="device" placeholder="设备ID" value="` + device + `" size="10" readonly>
	</div> 状态：
	`
				if err != nil {
					html += fmt.Sprint(err) + "<br>"
				} else {
					if deviceStatus.online {
							if len(deviceStatus.switches) > 1 {
								html += `<div style="display: inline-block;">`
								for i, s := range deviceStatus.switches {
									html += `
		<form name="Form_` + device + "_" + fmt.Sprint(i) + `" method="post" style="display: inline-block;">
			<input name="device" value="` + device + `" type="hidden">
			通道<input name="outlet" value="` + fmt.Sprint(i) + `" style="width: 25;" readonly>
			` + s + `
			<button name="action" value="on"`
									if s != "off" {
										html += " disabled"
									}
									html += `>开</button>
			<button name="action" value="off"`
									if s != "on" {
										html += " disabled"
									}
									html += `>关</button>
		</form><br>`
								}
								html = html[0:len(html)-4]
								html += `
	</div><br>`
							} else {
							//conlog("a\n" + alertlog(fmt.Sprint(status)) + "\nb")
								html += deviceStatus.switches[0] + `
	<form name="Form_` + device + `" method="post" style="display: inline-block;">
		<input name="device" value="` + device + `" type="hidden">
		<button name="action" value="on"`
								if deviceStatus.switches[0] != "off" {
									html += " disabled"
								}
								html += `>开</button>
		<button name="action" value="off"`
								if deviceStatus.switches[0] != "on" {
									html += " disabled"
								}
								html += `>关</button>
	</form><br>`
						}
					} else {
						html += "不在线<br>"
					}
				}
			}
			htmlOutput(w, html, 200, nil)
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
	//adminuser, _ := readConfig("admin", "user", 1)
	//if user != adminuser {
	//	return false
	//}
	userid := findConfig("admin", "user", user)
	if userid[0] < 0 {
		return false
	}
	md5 := admin[pos1+1:pos2]
	t, _ := strconv.Atoi(admin[pos2+1:])
	nt := int(time.Now().Unix())
	if t < nt {
		return false
	}
	adminpass, _ := readConfig("admin", "pass", userid[0])
	if passHashCookie(user, adminpass, t) == md5 {
		return true
	} else {
		return false
	}
}
func passHashCookie(u string, p string, t int) string {
	return fmt.Sprintf("%x", md5.Sum([]byte(u + "@" + p + "(" + fmt.Sprint(t))))
}
func checkAdminShowLoginPage(w http.ResponseWriter, r *http.Request) bool {
	r.ParseForm()
	//fmt.Println(r)
	path := r.URL.Path
	data := r.Form

	html := ""
	admincookie1, err := r.Cookie("admin")
	admincookie := ""
	if err != nil {
		admincookie = ""
	} else {
		admincookie = admincookie1.Value
	}
	if admincookie != "" {
		if checkCookie(admincookie) {
			return true
		}
	}
	if r.Method == "POST" && data.Get("user") != "" && data.Get("pass") != "" {
		//adminuser, _ := readConfig("admin", "user", 1)
		id := findConfig("admin", "user", data.Get("user"))
		if id[0] > 0 {
			adminpass, _ := readConfig("admin", "pass", id[0])
			if data.Get("pass") == adminpass {
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
				conlog("Admin login Success\n")
				return false
			}
		}
		html = `<meta http-equiv="refresh" content="3;URL=` + path + `">Failed`
		htmlOutput(w, html, 403, nil)
		conlog("Admin login failed\n")
		return false
	} else {
		conlog("Login page\n")
		html = `登录
<form action="" method="post" name="form1">
	username: <input name="user" type="text"><br>
	password: <input name="pass" type="password"><br>
	<button>提交</button>
<form>
<script>
	document.form1.user.focus();
</script>`
		htmlOutput(w, html, 401, nil)
	}
	return false
}
func oauthroute(w http.ResponseWriter, r *http.Request) {
	quit <- 0
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
		//data:image/png;base64,
		png := "iVBORw0KGgoAAAANSUhEUgAAACgAAAAYCAYAAACIhL/AAAAAAXNSR0IArs4c6QAAAoxJREFUWEftlk1IFGEYx3/PrGaXQkU6hOi4s6mHoEAqOnTykERCFztUl0ioJDp08uCtU6cSooQKAoMuUUEURgV6q8DoGLTzrpEIUmgddkFc58kZZ5dpcz9gV/Pg3Obl/z7v7/0/HzPCFn9ki/OxDVhthjbVQU0kdpPNHkVkRwj+i+bm9zI9vVzsIpsLCILjXEH1JhADMnjeCZmZmdoSgD6E9vTUs7h4CxgKoR7Q1nZJJiez60FuqoM5ALXtbmKxd6juBb4DvWLM17KA2tW1i+XlQVR9+8cllZqPbgpqyPMOAceARkRcVCcwxhXwKm0ITSQa8LxxYCDYozogqdSTkoDa2dlCNvsSOBxuMoj0RW+mjnMV1dF/AolMIXJBkkm3YkjHGVk14nqovyHGDJcGtO3jWNZEgWhIjLmbT00xwDXBT1TPSCr1phLIvy4rcg/XvSighXvzNajxeC/wOuyunK4Y4O8ACNqA+khQf/2sGONnouRTkI1XpNOnZX4+XRxwrb4eAf2h6DOW1S/J5GzewY6OdmKxjCSTP/IdubBwCpE7QEuoKwup/riJx29HOnlMjLlcvklseydwhLo6IZP5KHNzmXJOBKDx+D7gKbA/1JdMt9q2jWW9XW1Ep+IarASklEYTiVY87wVwMNRlEBmmsXEs+qUIJ0E0UyuInBTXLaz/IExN52DozLMIpH/GN+AxIh9QPbD6fh5oj1z2E5bVlyubojVYrYP5Ol1LXyFksfC+e4Piug+LCWrqYB7Scfagej/ScOudvwKM0tQ0/F9+FtRvOJFziIwUpDSX9msY87zcF2hDHIzaFYyU1tYmGhq6g/WlpS/Mzi6uN5TLjpla1WEt42y4g9XCbgNW6+AfkjjwGQmEbKUAAAAASUVORK5CYII="
		png1, _ := base64.StdEncoding.DecodeString(png)
		binOutput(w, png1, 200, nil)
		return
	}
	fmt.Print("\r")
	pass, _ := readConfig("admin", "pass", 0)
	//fmt.Println(pass)
	if pass == "" {
		if data.Get("adminuser") != "" && data.Get("adminpass") != "" {
			conlog("  Setting admin\n")
			values := make(map[string]string)
			values["user"] = data.Get("adminuser")
			values["pass"] = data.Get("adminpass")
			err := saveConfig("admin", values, 0)
			if err != nil {
				conlog(fmt.Sprintln("  Set admin failed\n", err))
				html := `failed
<meta http-equiv="refresh" content="5;URL=">
`
				htmlOutput(w, html, 400, nil)
			} else {
				conlog("  Set admin success\n")
				html := `Success
<meta http-equiv="refresh" content="3;URL=">
`
				htmlOutput(w, html, 201, nil)
			}
		} else {
			conlog("  Set admin first\n")
			html := `
设置管理员：
<form action="" method="post" name="form1">
	username: <input name="adminuser" type="text"><br>
	password: <input name="adminpass" type="password"><br>
	<button>提交</button>
<form>
<script>
	document.form1.adminuser.focus();
</script>
`
			htmlOutput(w, html, 201, nil)
		}
		return
	}
	if checkAdminShowLoginPage(w, r) {
		if query.Get("install") == "finish" {
			conlog("Adding devices\n")
			id_token := findConfig("token", "clientID", query.Get("tokenID"))
			if id_token[0] < 0 {
				html := "Something error finding clientID: " + query.Get("tokenID")
				htmlOutput(w, html, 400, nil)
			}
			tokenInit(id_token[0])
			var err error
			//fmt.Println(data["deviceID[]"])
			for _, deviceID := range data["deviceID[]"] {
				if deviceID != "" {
					//fmt.Println(deviceID)
					id_devices := findConfig("device", "deviceID", deviceID)
					id_device := id_devices[0]
					if id_device < 0 {
						id_device = 0
					}
					values := make(map[string]string)
					values["tokenID"] = strconv.Itoa(id_token[0])
					values["deviceID"] = deviceID
					err = saveConfig("device", values, id_device)
				}
			}
			if err != nil {
				conlog("  Error\n")
				html := "Something error in saving: " + fmt.Sprint(err)
				htmlOutput(w, html, 400, nil)
			} else {
				html := `成功，<br>请关闭窗口。`
				htmlOutput(w, html, 200, nil)
				conlog("  Success\n")
				// 成功，结束Oauth
				quit <- -1
			}
			return
		}
		if query.Get("install") == "addDevice" {
			conlog("Add devices\n")
			id_token, _ := strconv.Atoi(query.Get("tokenID"))
			html := `
<form action="?install=finish&tokenID=` + query.Get("tokenID") + `" method="post" name="form1">
	自动获取到的：
	`
			deviceIDs, err := listDevices(id_token)
			//"thingList":[],"total":0
			if err != nil {
				fmt.Println(err)
			} else {
				//fmt.Println(deviceIDs)
				for _, deviceID := range deviceIDs {
					html += `Device ID: <input type="text" name="deviceID[]" value="` + deviceID + `"><br>`
				}
			}
			html += `
如果自动获取不到，可能是因为不是特定品牌的设备，所以列不出来。<br>
【<a href="https://ewelink.gitee.io/ewelink-api/#/zh-cmn/%E6%94%B6%E8%B4%B9%E6%A0%87%E5%87%86" target="_blank">对于企业或者个人开发者申请的 APPID，目前开放 部分已授权品牌的设备、以及主流设备类型</a>】<br>
手动输入设备ID（在易微联APP或小程序里查看）：<br>
	Device ID: <button onclick="addButton(); return false;">再添加一行</button><br>
	<span id="inputs"></span>
	<button>提交</button>
<form>
<script>
	var area = document.getElementById("inputs");
	function addButton() {
		let input = document.createElement("input");
		input.name = "deviceID[]";
		input.type = "text";
		area.appendChild(input);
		area.appendChild(document.createElement("br"));
	}
	addButton();
</script>
`
			htmlOutput(w, html, 200, nil)
			return
		}
		if query.Get("code") != "" {
			// 有code
			conlog("  Received a code\n")
			ids := findConfig("client", "appID", query.Get("state"))
			id := ids[0]
			if id == -1 {
				html := "Something error finding appID: " + query.Get("appID")
				htmlOutput(w, html, 400, nil)
			}
			appInit(id)
			data1 := "{\"code\":\"" + query.Get("code") + "\", \"redirectUrl\":\"" + oauthapp.redirectUrl + "\", \"grantType\":\"authorization_code\"}"
			head := make(map[string]string)
			head["X-CK-Appid"] = oauthapp.appID
			head["Content-Type"] = "application/json"
			head["Authorization"] = "Sign " + ComputeHmac256(data1, oauthapp.appSecret)
			head["Host"] = ewelinkapi.host

			res, err := curl("POST", ewelinkapi.scheme + ewelinkapi.host + ewelinkapi.oauthToken, data1, head)
			if err != nil {
				htmlOutput(w, fmt.Sprint(head, err), 400, nil)
			} else {
				//res.StatusCode
				conlog("  get access token success\n")
				body := res.Body
				accessToken := readValueInString(string(body), "accessToken")
				atet := readValueInString(string(body), "atExpiredTime")
				rt := readValueInString(string(body), "refreshToken")
				rtet := readValueInString(string(body), "rtExpiredTime")
				conlog("  saving access token\n")
				values := make(map[string]string)
				values["accessToken"] = accessToken
				values["atExpiredTime"] = atet
				values["refreshToken"] = rt
				values["rtExpiredTime"] = rtet
				values["clientID"] = strconv.Itoa(id)
				err := saveConfig("token", values, 0)
				if err != nil {
					conlog("  saving access token failed\n")
					htmlOutput(w, "saving access token failed", 400, nil)
				} else {
					ids := findConfig("token", "accessToken", accessToken)
					id = ids[0]
					if id == -1 {
						html := "Something error finding accessToken ."
						htmlOutput(w, html, 400, nil)
					}
					html := `Success
<meta http-equiv="refresh" content="3;URL=?install=addDevice&tokenID=` + strconv.Itoa(id) + `">
`
					htmlOutput(w, html, 200, nil)
				}
			}
			return
		}
		if query.Get("install") == "2" {
			id := -1
			if data.Get("app") != "" {
				// 用已有app，不用保存，确认一下存在
				ids := findConfig("client", "id", data.Get("app"))
				id = ids[0]
				if id == -1 {
					html := "Something error in POST: " + data.Get("app")
					htmlOutput(w, html, 400, nil)
					return
				}
			} else {
				// 保存新app
				values := make(map[string]string)
				values["appID"] = data.Get("appID")
				values["appSecret"] = data.Get("appSecret")
				values["appRedirect"] = data.Get("redirectUrl")
				ids := findConfig("client", "appID", values["appID"])
				id = ids[0]
				if id == -1 {
					id = 0
				}
				err := saveConfig("client", values, id)
				if err != nil {
					html := "Something error in saving: " + fmt.Sprint(err)
					htmlOutput(w, html, 400, nil)
					return
				}
				if id == 0 {
					ids := findConfig("client", "appID", values["appID"])
					id = ids[0]
					if id == -1 {
						html := "Something error in find: " + values["appID"]
						htmlOutput(w, html, 400, nil)
						return
					}
				}
			}
			conlog("  redirecting to Ewelink\n")
			appInit(id)
			time1 := time.Now().Unix() * 1000
			signstr := ComputeHmac256(oauthapp.appID + "_" + fmt.Sprint(time1), oauthapp.appSecret)
			url := "https://c2ccdn.coolkit.cc/oauth/index.html" +
			"?clientId=" + oauthapp.appID +
			"&authorization=" + signstr +
			"&seq=" + fmt.Sprint(time1) +
			"&redirectUrl=" + oauthapp.redirectUrl +
			"&nonce=ysun1234" +
			"&grantType=authorization_code" +
			"&state=" + oauthapp.appID
			html := `redirecting
<script>
	location.href = "` + url + `";
</script>`
			//fmt.Fprint(w, html)
			htmlOutput(w, html, 200, nil)
			return
		}
		if query.Get("install") == "1" {
			conlog("OAuth Page Start\n")
			html1 := `
1. 在 <a href="https://dev.ewelink.cc/" target="_blank">https://dev.ewelink.cc/</a> 登录，申请成为开发者（可能要等几天）<br>
2. 在 <a href="https://dev.ewelink.cc/#/console" target="_blank">https://dev.ewelink.cc/#/console</a> 新建一个应用（个人开发者只能创建一个），将跳转地址设为下面 Redirect URL 中的url（其实就是当前页面）<br>
3. 将 APPID 与 APP SECRET 填入下方，点击按钮提交给程序<br>
	App ID: <input name="appID" type="text"><br>
	App Secret: <input name="appSecret" type="password"><br>
	Redirect URL: <input name="redirectUrl" type="text" readonly><br>
`
			html := `<form action="?install=2" method="post" name="form1" onsubmit="return check(this);">
`
			result, err := readConfig("client", "id,appID", 0)
			if err == nil && result != "" {
				apps := strSplitLine(result)
				for _, app := range apps {
					id := app[0:strings.Index(app, "|")]
					appID := app[strings.Index(app, "|")+1:]
					html += `<label>
	<input type="radio" name="app" value="` + id + `">使用已有App ID: ` + appID + `<br>
</label><br>
`
				}
				html += `<label>
	<input type="radio" name="app" value="">或新输入一个App:<br>
` + html1 + `
</label>
`
			} else {
				html += html1
			}
			html += `
	<button>提交</button>
<form>
<script>
	let url = location.href;
	url = url.substr(0, url.indexOf("?"));
	document.form1.redirectUrl.value = url;
	function check(f) {
		let e = document.getElementsByName("app");
		for (let i=0; i<e.length; i++) {
			if (e[i].checked) {
				if (e[i].value != "") return true;
			}
		}
		if (f.appID.value != "" && f.appSecret.value != "") {
			return true;
		} else {
			alert("请输入");
			return false;
		}
	}
</script>`
			htmlOutput(w, html, 201, nil)
			return
		}
		conlog("  Add account\n")
		html := "<a href=\"?install=1\">认证新账号</a>（注意：账号授权认证将会使这个账号在此处与其它地方的任何原有token失效！）<br>"
		dIDs, _ := readConfig("device", "deviceID", 0)
		//fmt.Println(dIDs)
		if dIDs != "" {
			result, err := readConfig("token", "id", 0)
			if err == nil && result != "" {
				html += `<br>或者在已有账号中添加设备：<br>
<form action="" method="post" name="form1" onsubmit="return check(this);">`
				id_tokens := strSplitLine(result)
				for _, id_token := range id_tokens {
					html += `
	<label>
		<input type="radio" name="tokenID" value="` + id_token + `">账号` + id_token + `，其中已有的设备ID: `
					sql := "select deviceID from device where tokenID=" + id_token + ";"
					result, err = sqlite(sql)
					if err == nil && result != "" {
						deviceIDs := strSplitLine(result)
						for _, deviceID := range deviceIDs {
							html += deviceID + ", "
						}
						html = html[0:len(html)-2]
					}
					html += `
	</label><br>`
				}
				html += `
	<button>确定</button><br>
<form>
<script>
	function check(f) {
		let e = document.getElementsByName("tokenID");
		for (let i=0; i<e.length; i++) {
			if (e[i].checked) {
				f.action = "?install=addDevice&tokenID=".concat(e[i].value);
				return true;
			}
		}
		return false;
	}
</script>`
			}
		}
		htmlOutput(w, html, 201, nil)
		return
	}
}

func turnLight(deviceID string, turn string) error {
	if turn != "on" && turn != "off" {
		return errors.New("Invalid operate: \"" + turn + "\"")
	}
	port := -1
	if strings.Index(deviceID, ":") > 0 {
		outlet := deviceID[strings.Index(deviceID, ":")+1:]
		port1, err := strconv.Atoi(outlet)
		//fmt.Println(port1)
		if err != nil {
			return errors.New("Outlet: \"" + outlet + "\" not number.")
		}
		port = port1
		//fmt.Println(port)
		deviceID = deviceID[0:strings.Index(deviceID, ":")]
	}
	// 尝试查找有没有这个device
	id_devices := findConfig("device", "deviceID", deviceID)[0]
	if id_devices < 1 {
		// 没有这个device，可能参数传入的是序列id
		id_device, err := strconv.Atoi(deviceID)
		if err != nil {
			// 没有这个device，也不是 数字
			return errors.New("device ID: \"" + deviceID + "\" NOT found.")
		}
		// 尝试读取这个数字对应的deviceID
		deviceID1, err := readConfig("device", "deviceID", id_device)
		if err != nil || deviceID1 == "" {
			// 没有这个数字对应的device
			return errors.New("device ID: \"" + deviceID + "\" NOT found.")
		}
		deviceID = deviceID1
	}
	return setDeviceStatus(turn, deviceID, port)
}

func getUserProfile(id int) (string, error) {
	head := make(map[string]string)
	head["X-CK-Appid"] = oauthapp.appID
	head["Host"] = ewelinkapi.host
	head["Content-Type"] = "application/json"
	head["Authorization"] = "Bearer " + oauthapp.accessToken
	url := ewelinkapi.scheme + ewelinkapi.host + "/v2/user/profile"
	res, err := curl("GET", url, "", head)
	if err != nil {
		return res.Body, err
	} else {
		body := res.Body
		//fmt.Println(body)
		err1 := readValueInString(string(body), "error")
		if err1 != "0" {
			conlog("  get user profile failed.\n")
			return "", errors.New(body)
		} else {
			return body, nil
		}
	}
}
func getFamily(id int) (string, error) {
	head := make(map[string]string)
	head["X-CK-Appid"] = oauthapp.appID
	head["Host"] = ewelinkapi.host
	head["Content-Type"] = "application/json"
	head["Authorization"] = "Bearer " + oauthapp.accessToken
	url := ewelinkapi.scheme + ewelinkapi.host + "/v2/family"
	res, err := curl("GET", url, "", head)
	if err != nil {
		return res.Body, err
	} else {
		body := res.Body
		//fmt.Println(body)
		err1 := readValueInString(string(body), "error")
		if err1 != "0" {
			conlog("  get user family failed.\n")
			return "", errors.New(body)
		} else {
			return body, nil
		}
	}
}
func listDevices(tokenID int) ([]string, error) {
	tokenInit(tokenID)
	var result []string
	head := make(map[string]string)
	head["X-CK-Appid"] = oauthapp.appID
	head["Host"] = ewelinkapi.host
	head["Content-Type"] = "application/json"
	head["Authorization"] = "Bearer " + oauthapp.accessToken
	url := ewelinkapi.scheme + ewelinkapi.host + "/v2/device/thing"
	res, err := curl("GET", url, "", head)
	if err != nil {
		return result, err
	} else {
		body := res.Body
		//fmt.Println(body)
		err1 := readValueInString(string(body), "error")
		if err1 != "0" {
			conlog("  list device failed.\n")
			return result, errors.New(body)
		} else {
			dIDs := body[strings.Index(body, "deviceid")-1:]
			for strings.Index(dIDs, "deviceid") > 0 {
				deviceID := readValueInString(string(dIDs), "deviceid")
				result = append(result, deviceID)
				dIDs = dIDs[strings.Index(dIDs, "deviceid")+6:]
			}
			return result, nil
		}
	}
}
func findTokenIDofDevice(deviceID string) (int, error) {
	id_device := findConfig("device", "deviceID", deviceID)
	if id_device[0] < 0 {
		return -1, errors.New("device ID " + deviceID + " not found.")
	}
	id_token_string, err := readConfig("device", "tokenID", id_device[0])
	if err != nil {
		return -1, err
	}
	id_token, err := strconv.Atoi(id_token_string)
	if err != nil {
		return -1, err
	}
	return id_token, nil
}
func checkDeviceOnline(deviceID string) (DeviceStatus, error) {
	id_token, _ := findTokenIDofDevice(deviceID)
	tokenInit(id_token)
	var result DeviceStatus
	head := make(map[string]string)
	head["X-CK-Appid"] = oauthapp.appID
	head["Host"] = ewelinkapi.host
	head["Content-Type"] = "application/json"
	head["Authorization"] = "Bearer " + oauthapp.accessToken
	url := ewelinkapi.scheme + ewelinkapi.host + "/v2/device/thing"
	data := `{"thingList": [{"itemType": 1, "id": "` + deviceID + `"}]}`
	res, err := curl("POST", url, data, head)
	if err != nil {
		return result, err
	} else {
		body := res.Body
		//fmt.Println(body)
		err1 := readValueInString(string(body), "error")
		if err1 != "0" {
			conlog("  check device online failed.\n")
			return result, errors.New(body)
		} else {
			deviceid := readValueInString(string(body), "deviceid")
			if deviceid == "" {
				return result, errors.New("设备ID " + deviceID + " 可能不属于你。" + body)
			} else {
				result.deviceID = deviceid
			}
			name := readValueInString(string(body), "name")
			if name == "" {
				return result, errors.New("设备ID " + deviceID + " 可能不属于你。" + body)
			} else {
				result.name = name
			}
			online := readValueInString(string(body), "online")
			if online == "true" {
				result.online = true
			}
			if online == "false" {
				result.online = false
			}
			switches := readValueInString(string(body), "switches")
			//fmt.Println(switches)
			var switch1 []string
			if switches != "" {
				switches = body[strings.Index(body, "switches")+8:]
				switches = switches[0:strings.Index(switches, "]")]
				for strings.Index(switches, "switch") > 0 {
					switch0 := readValueInString(string(switches), "switch")
					switch1 = append(switch1, switch0)
					switches = switches[strings.Index(switches, "switch")+6:]
				}
				//fmt.Println(result)
			} else {
				status := readValueInString(string(body), "switch")
				switch1 = append(switch1, status)
			}
			result.switches = switch1
			return result, nil
		}
	}
}
func getDeviceStatus(deviceID string) ([]string, error) {
	var result []string
	id_token, err := findTokenIDofDevice(deviceID)
	if err != nil {
		return result, err
	}
	tokenInit(id_token)
	//fmt.Println("token id ", id_token)
	head := make(map[string]string)
	head["X-CK-Appid"] = oauthapp.appID
	head["Host"] = ewelinkapi.host
	head["Content-Type"] = "application/json"
	head["Authorization"] = "Bearer " + oauthapp.accessToken
	url := ewelinkapi.scheme + ewelinkapi.host + "/v2/device/thing/status" + "?type=1&id=" + deviceID + "&params=switch%7Cswitches"
	res, err := curl("GET", url, "", head)
	if err != nil {
		return result, err
	} else {
		body := res.Body
		//fmt.Println(body)
		err1 := readValueInString(string(body), "error")
		if err1 != "0" {
			conlog("  get " + deviceID + " status failed.\n")
			return result, errors.New(body)
		} else {
			switches := readValueInString(string(body), "switches")
			//fmt.Println(switches)
			if switches != "" {
				switches = body[strings.Index(body, "switches")+8:]
				for strings.Index(switches, "switch") > 0 {
					switch0 := readValueInString(string(switches), "switch")
					result = append(result, switch0)
					switches = switches[strings.Index(switches, "switch")+6:]
				}
				//fmt.Println(result)
				return result, nil
			} else {
				status := readValueInString(string(body), "switch")
				result = append(result, status)
				return result, nil
			}
		}
	}
}
func setDeviceStatus(status string, deviceID string, port int) error {
	deviceStatus, err := checkDeviceOnline(deviceID)
	if err != nil {
		return err
	}
	if ! deviceStatus.online {
		return errors.New("Device " + deviceID + " is offline.")
	}
	head := make(map[string]string)
	head["X-CK-Appid"] = oauthapp.appID
	head["Host"] = ewelinkapi.host
	head["Content-Type"] = "application/json"
	head["Authorization"] = "Bearer " + oauthapp.accessToken
	url := ewelinkapi.scheme + ewelinkapi.host + "/v2/device/thing/status"
	data := ""
	if len(deviceStatus.switches) > 1 {
		if port < 0 {
			port = 0
		}
		data = "{\"type\":1,\"id\":\"" + deviceID + "\",\"params\":{\"switches\":[{\"switch\":\"" + status + "\", \"outlet\": " + strconv.Itoa(port) + "}]}}"
	} else {
		data = "{\"type\":1,\"id\":\"" + deviceID + "\",\"params\":{\"switch\":\"" + status + "\"}}"
	}
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
func RefreshToken(id int) error {
	refreshToken, err := readConfig("token", "refreshToken", id)
	if err != nil {
		return err
	}
	data := "{\"rt\":\"" + refreshToken + "\"}"
	head := make(map[string]string)
	head["X-CK-Appid"] = oauthapp.appID
	head["Host"] = ewelinkapi.host
	head["Content-Type"] = "application/json"
	head["Authorization"] = "Bearer " + oauthapp.accessToken

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
			accessToken := readValueInString(string(body), "at")
			atet := (time.Now().Unix() +  30 * 24 * 60 * 60) * 1000
			rt := readValueInString(string(body), "rt")
			rtet := (time.Now().Unix() +  60 * 24 * 60 * 60) * 1000
			values := make(map[string]string)
			values["accessToken"] = accessToken
			values["atExpiredTime"] = strconv.FormatInt(atet, 10)
			values["refreshToken"] = rt
			values["rtExpiredTime"] = strconv.FormatInt(rtet, 10)
			return saveConfig("token", values, id)
		}
	}
}
	//content, err := ioutil.ReadFile(dbFilePath)
	//return ioutil.WriteFile(dbFilePath, []byte(str), 0666)
func sqlite(str string) (string, error) {
	//fmt.Println(str)
	result := ""
	cmd := exec.Command("sqlite3", dbFilePath, str)
	/*stdout, err := cmd.StdoutPipe()
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
	return result, nil*/
	result_b, err := cmd.Output()
	result = strings.TrimSpace(string(result_b))
	result = strings.TrimRight(result, "\n")
	return result, err
}
func saveConfig(table string, key_value map[string]string, id int) error {
	if !validSqlKey(table) {
		return errors.New("\"" + table + "\" is invalid.")
	}
	for key, value := range key_value {
		if !validSqlKey(key) {
			return errors.New("\"" + key + "\" is invalid.")
		}
		if !validSqlKey(value) {
			return errors.New("\"" + value + "\" is invalid.")
		}
	}
	oldvalue, err := readConfig(table, "*", id)
	if err == nil {
		if id == 0 || oldvalue == "" {
			keys := ""
			values := ""
			for key, value := range key_value {
				keys += key + ", "
				values += "\"" + value + "\", "
			}
			keys = keys[0:strings.LastIndex(keys, ",")]
			values = values[0:strings.LastIndex(values, ",")]
			sql := "insert into " + table + " (" + keys + ") values (" + values + ");"
			//fmt.Println(sql)
			_, err = sqlite(sql)
		} else {
			keys := ""
			for key, value := range key_value {
				keys += key + "=\"" + value + "\", "
			}
			keys = keys[0:strings.LastIndex(keys, ",")]
			sql := "update " + table + " set " + keys + " where id=" + strconv.Itoa(id) + ";"
			//fmt.Println(sql)
			_, err = sqlite(sql)
		}
	}
	return err
}
func readConfig(table string, key string, id int) (string, error) {
	if !validSqlKey(table) {
		return "", errors.New("\"" + table + "\" is invalid.")
	}
	if !validSqlKey(key) {
		return "", errors.New("\"" + key + "\" is invalid.")
	}
	if id < 0 {
		return "", errors.New("id is invalid.")
	}
	sql := "select " + key + " from " + table
	if id > 0 {
		sql += " where id=" + strconv.Itoa(id)
	}
	sql += ";"
	//fmt.Println(sql)
	return sqlite(sql)
}
func findConfig(table string, key string, value string) []int {
	var ids []int
	if !validSqlKey(table) {
		ids = append(ids, -1)
		return ids
	}
	if !validSqlKey(key) {
		ids = append(ids, -1)
		return ids
	}
	if !validSqlKey(value) {
		ids = append(ids, -1)
		return ids
	}
	sql := "select id from " + table + " where " + key + "=\"" + value + "\";"
	id_string, err := sqlite(sql);
	if err != nil || id_string == "" {
		ids = append(ids, -1)
		return ids
	}
	id_arr := strSplitLine(id_string)
	for _, id1 := range id_arr {
		id, err := strconv.Atoi(id1)
		if err != nil {
			var ids1 []int
			ids1 = append(ids1, -1)
			return ids1
		} else {
			ids = append(ids, id)
		}
	}
	return ids
}
func validSqlKey(str string) bool {
	if str == "" {
		return false
	}
	tmp := strings.Index(str, " ")
	if tmp > -1 {
		return false
	}
	tmp = strings.Index(str, ";")
	if tmp > -1 {
		return false
	}
	return true
}

func validToken(id int) bool {
	//fmt.Println("_" + accessToken + "_")
	atExpiredTime, _ := readConfig("token", "atExpiredTime", id)
	if len(atExpiredTime) > 3 {
		atExpiredTime = atExpiredTime[0:len(atExpiredTime)-3]
	}
	time1, err := strconv.Atoi(atExpiredTime)
	if err != nil {
		fmt.Println(err)
	}
	time2 := int(time.Now().Unix() + 15*24*60*60)
	if time1 < time2 {
		err = RefreshToken(id)
		if err != nil {
			conlog(alertlog(fmt.Sprint(err)) + "\n")
			return false
		}
	}
	//_, err = listDevices()
	_, err = getFamily(id)
	if err != nil {
		return false
	} else {
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
func binOutput(w http.ResponseWriter, body []byte, code int, head map[string]string) {
	//w.Header().Set("Content-Type", "application/stream")
	w.Header().Set("Content-Type", "image/png")
	for k, v := range head {
		w.Header().Add(k, v)
	}
	w.WriteHeader(code)
	w.Write([]byte(body))
}
func htmlOutput(w http.ResponseWriter, body string, code int, head map[string]string) {
	w.Header().Set("Content-Type", "text/html")
	for k, v := range head {
		w.Header().Add(k, v)
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
		header["User-Agent"] = "ysun"
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
