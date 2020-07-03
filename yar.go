package yar

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
)

const headerSize  int = 82
const magicNum uint32=0x80DFEC60

type onFunc func(c interface{})interface{}
type errorHandle func(error)

type config struct {
	network string
	address string
	funcList  map[string] onFunc
	errorList []errorHandle
}

type Yaf interface {
	OnError(errorHandle)
	On(string,onFunc) error
	Run() error
}

type header struct {
	id			uint32 	// transaction id
	version 	uint16  	// protocl version
	magicNum	uint32	// default is: 0x80DFEC60
	reserved	uint32
	provider	string  // reqeust from who
	token		string	// request token, used for authentication
	bodyLen		uint32	// request body len
}

type body struct {
	Id uint32			`json:"i"`
	Method string		`json:"m"`
	Param []interface{}	`json:"p"`
}

func (c config) OnError(h errorHandle) {
	c.errorList=append(c.errorList,h)
}

func (c config) On(methodName string , h onFunc) error {
	c.funcList[methodName]=h
	return nil
}

func yarErr(c config,err error) {
	fmt.Println(`yarErr:`,err.Error())
	for _,v:= range c.errorList {
		v(err)
	}
}

func (c config) Run() error {
	ln,err:=net.Listen(c.network,c.address)
	if err!=nil{
		return err
	}
	fmt.Println("正在监听9002")
	for true {
		conn, err := ln.Accept()
		fmt.Println(`for`)
		if err != nil {
			yarErr(c,err)
		}
		go handle(conn,c)
	}
	return nil
}

func Addr(network string,address string) Yaf {
	var y Yaf
	var fl=make(map[string]onFunc)
	var eh = make([]errorHandle,0)
	y=config{network,address,fl,eh}
	return y
}

func handle(conn net.Conn,c config){
	//自动关闭
	fmt.Println(`handle()`)
	yarErr(c,errors.New(`yarErr`))
	defer func() {
		err:=conn.Close()
		if err != nil {
			yarErr(c,errors.New("net.Conn error: "+err.Error()))
			return
		}
		fmt.Println(`conn.Close()`)
	}()

	b,err:=parseRequest(conn)
	if err != nil {
		yarErr(c,err)
		return
	}
	fmt.Println(b)
	f,ok:=c.funcList[b.Method]
	if ok {
		p:=f(b.Param)
		var res=make(map[string]interface{})
		var o= make([]interface{},0)
		o=append(o,p)
		res[`i`]=0
		res[`s`]=0
		res[`r`]=true
		r,err:=json.Marshal(res)
		if err != nil {
			yarErr(c,err)
			return
		}
		j:=make([]byte,8)
		j2:=[]byte(`JSON`)
		copy(j,j2)
		r=append(j,r...)
		h,err:=packHeader(len(r))
		fmt.Println(len(h))
		fmt.Println(len(r))
		if err != nil {
			yarErr(c,err)
			return
		}
		h=append(h,r...)
		fmt.Println(string(h))
		lll, err := conn.Write(h)
		if err!=nil{
			fmt.Println(err.Error())
		}
		fmt.Println(lll)
		return
	}else{
		yarErr(c,err)
		return
	}
}

func parseRequest(conn net.Conn) (body,error) {
	var b body
	bodyLen,err:=validRequest(conn)
	if err != nil {
		return b,err
	}

	bodyByte := make([]byte,bodyLen)
	_,err = conn.Read(bodyByte)
	if err != nil {
		return b,errors.New("request read error: "+err.Error())
	}
	fmt.Println(string(bodyByte))
	switch strings.TrimRight(string(bodyByte[0:8]),"\000") {
	case `JSON`:
		err=json.Unmarshal(bodyByte[8:], &b)
	default:
		err=errors.New(`unsupported packager`)
	}
	if err != nil {
		return b,err
	}
	return b,nil
}


func validRequest(conn net.Conn) (uint32,error) {
	var hand header
	handByte := make([]byte, headerSize)
	_, err := conn.Read(handByte)
	if err != nil {
		return 0,errors.New("request read error: "+err.Error())
	}
	fmt.Println(string(handByte))

	hand,err=unpackHeader(handByte)
	if err != nil {
		return  0,err
	}

	if hand.magicNum!=magicNum{
		return  0,errors.New("illegal Yar RPC request")
	}

	return hand.bodyLen,nil
}

func unpackHeader(b []byte) (header,error) {
	var h=header{}
	if len(b)!=headerSize{
		return h,errors.New("unpackHeader error: header bytes len not "+string(headerSize))
	}
	var err error
	h.id,err=bytesToUint32(b[0:4])
	if err!=nil {
		return h,errors.New("unpackHeader error: "+err.Error())
	}
	h.version,err=bytesToUint16(b[4:6])
	if err!=nil {
		return h,errors.New("unpackHeader error: "+err.Error())
	}
	h.magicNum,err=bytesToUint32(b[6:10])
	if err!=nil {
		return h,errors.New("unpackHeader error: "+err.Error())
	}
	h.reserved,err=bytesToUint32(b[10:14])
	if err!=nil {
		return h,errors.New("unpackHeader error: "+err.Error())
	}
	h.provider=strings.TrimRight(string(b[14:46])," ")
	h.token=strings.TrimRight(string(b[46:78])," ")
	h.bodyLen,err=bytesToUint32(b[78:82])
	if err!=nil {
		return h,errors.New("unpackHeader error: "+err.Error())
	}
	fmt.Println(h)
	return h,nil
}

func packHeader(len int) ([]byte,error) {
	//$bin = pack("NnNNA32A32N",
	//$id, 0, 0x80DFEC60,
	//	0, "Yar PHP TCP Server",
	//	"", $len
	//);

	//id 32
	var b = make([]byte,0)
	id,err:=uint32ToBytes(0)
	if err!=nil {
		return b,errors.New("packHeader error: "+err.Error())
	}
	b=append(b,id...)

	//version 16
	res,err:=uint16ToBytes(0)
	if err!=nil {
		fmt.Println(err.Error())
	}
	b=append(b,res...)

	//magic_num 32
	res,err=uint32ToBytes(magicNum)
	if err!=nil {
		fmt.Println(err.Error())
	}
	b=append(b,res...)

	//reserved 32
	res,err=uint32ToBytes(0)
	if err!=nil {
		fmt.Println(err.Error())
	}
	b=append(b,res...)

	//provider 32
	var provider=[]byte(`Yar PHP TCP Server`)
	res =make([]byte,32)
	copy(res,provider)
	b=append(b,res...)

	//token 32
	var token=[]byte(``)
	res =make([]byte,32)
	copy(res,token)
	b=append(b,res...)

	//body_len 32
	res,err=uint32ToBytes(uint32(len))
	if err!=nil {
		fmt.Println(err.Error())
	}
	b=append(b,res...)

	fmt.Println(`b:`,b)
	fmt.Println(string(b))

	_,err=unpackHeader(b)
	if err!=nil {
		fmt.Println(err.Error())
	}

	return b,nil
}


func bytesToUint32(b []byte) (uint32,error) {
	bytesBuffer := bytes.NewBuffer(b)

	var x uint32
	err:=binary.Read(bytesBuffer, binary.BigEndian, &x)
	if err!=nil {
		return 0, errors.New("bytesToUint32 error: " + err.Error())
	}
	return x,nil
}

//字节转换成整形
func bytesToUint16(b []byte) (uint16,error) {
	bytesBuffer := bytes.NewBuffer(b)

	var x uint16
	err:=binary.Read(bytesBuffer, binary.BigEndian, &x)
	if err!=nil {
		return 0, errors.New("bytesToUint16 error: " + err.Error())
	}
	return x,nil
}

func uint32ToBytes(u uint32) ([]byte,error)  {
	bytesBuffer := bytes.NewBuffer([]byte{})
	err:=binary.Write(bytesBuffer, binary.BigEndian, u)
	if err!=nil {
		return []byte{}, errors.New("uint32ToBytes error: " + err.Error())
	}
	return bytesBuffer.Bytes(),nil
}

func uint16ToBytes(u uint16) ([]byte,error)  {
	bytesBuffer := bytes.NewBuffer([]byte{})
	err:=binary.Write(bytesBuffer, binary.BigEndian, u)
	if err!=nil {
		return []byte{}, errors.New("uint32ToBytes error: " + err.Error())
	}
	return bytesBuffer.Bytes(),nil
}