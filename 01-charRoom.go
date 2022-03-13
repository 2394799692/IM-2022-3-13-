package main

import (
	"fmt"
	"net"
	"strings"
	"time"
)

//创建用户结构体类型！
type Client struct {
	C    chan string
	Name string
	Addr string
}

//创建全局map，存储在线用户
var onlineMap map[string]Client

//创建全局channel,用于传递用户消息
var message = make(chan string)

func WriteMsgToClient(clnt Client, conn net.Conn) {
	//监听用户自带channel上是否有消息
	for msg := range clnt.C {
		conn.Write([]byte(msg + "\n"))
	}
}
func MakeMsg(clnt Client, msg string) (buf string) {
	buf = "[" + clnt.Addr + "]" + clnt.Name + ":" + msg
	return
}
func HandlerConnect(conn net.Conn) {
	defer conn.Close()
	//创建channel判断，用户是否活跃
	hasData := make(chan bool)
	//获取用户网络地址 ip+port
	netAddr := conn.RemoteAddr().String()
	//创建用户的结构体信息
	clnt := Client{make(chan string), netAddr, netAddr}
	//将新链接用户，添加到在线用户map中
	onlineMap[netAddr] = clnt
	//发送用户上线消息到全局channel中
	go WriteMsgToClient(clnt, conn)
	//创建专门用来给当前用户发送消息的go程
	message <- MakeMsg(clnt, "login")
	//创建一个channel，用来判断退出状态
	isQuit := make(chan bool)
	//创建一个匿名go程，专门处理用户发送的消息。
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := conn.Read(buf)
			if n == 0 {
				isQuit <- true
				fmt.Printf("检测到客户端：%s退出", clnt.Name)
				return
			}
			if err != nil {
				fmt.Println("conn.Read err:", err)
				return
			}
			//将读到的用户消息，写入到message中
			msg := string(buf[:n-1]) //减去1的意思是去掉斜杆
			//提取在线用户列表
			if msg == "who" && len(msg) == 3 {
				conn.Write([]byte("online user list:\n"))
				//遍历当前map、获取在线用户
				for _, user := range onlineMap {
					userInfo := user.Addr + ":" + user.Name + "\n"
					conn.Write([]byte(userInfo))
				}
			} else if len(msg) >= 8 && msg[:6] == "rename" {
				newName := strings.Split(msg, "|")[1]
				clnt.Name = newName       //修改结构体成员name
				onlineMap[netAddr] = clnt //更新onlineMap
				conn.Write([]byte("rename successful\n"))
			} else {
				//将读到的用户消息，写入到message中
				message <- MakeMsg(clnt, msg)
			}
			hasData <- true
		}
	}()
	//保证不退出
	for {
		//监听channel上的数据流动
		select {
		case <-isQuit:
			delete(onlineMap, clnt.Addr)       //将用户从online移除
			message <- MakeMsg(clnt, "logout") //写入用户退出消息到全局channel
			return
		case <-hasData:
			//什么都不做，目的是重置下面case的计时器。
		case <-time.After(time.Second * 10):
			delete(onlineMap, clnt.Addr)       //将用户从online移除
			message <- MakeMsg(clnt, "logout") //写入用户退出消息到全局channel
			return
		}
	}
}
func Manager() {
	//初始化map
	onlineMap = make(map[string]Client)
	for { //循环从message中读取
		//监听全局channel中是否有数据,有数据存储至msg，无数据阻塞
		msg := <-message
		//循环发送消息给所有在线用户
		for _, clnt := range onlineMap {
			clnt.C <- msg
		}
	}
}
func main() {
	//创建监听套接字
	listener, err := net.Listen("tcp", "127.0.0.1:8000")
	if err != nil {
		fmt.Println("Listen err", err)
		return
	}
	defer listener.Close()
	//循环监听客户端连接请求
	//创建管理者go程，管理map和ch
	go Manager()
	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Accept err", err)
			return
		}
		//启动go程处理客户端请求
		go HandlerConnect(conn)
	}
}
