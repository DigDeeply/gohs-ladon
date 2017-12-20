# gohs-Ladon
[![Build Status](https://travis-ci.org/DigDeeply/gohs-ladon.svg?branch=master)](https://travis-ci.org/DigDeeply/gohs-ladon)    
基于Intel开源的hyperscan实现的GO版本的一个服务，取名拉冬，希腊神话中表示百头巨龙。给定一行文本，能够从海量的正则表达式中快速查询出命中了哪些正则，还可以返回该正则附加的一些数据。

## 使用示例
例如，给出一个正则文本:
第一列是唯一id, 第二列是正则表达式, 第三列是附加数据
```
1	^[你|叫|什么|的|是]*名字[你|叫|什么|的|是]*$	{"type:"name", "user":"you"}
2	^[唱|一首|来]*歌[曲|吧|啊]*$	{"type":"song", "name":"random"}
3	^[唱|一首|来]*东风破[的|歌|歌曲|吧|啊]*$	{"type":"song", "name":"东风破"}
```
### 启动服务
```sh
./gohs-ladon --filepath=patterns/pattern2.txt
[2017-12-20T06:50:50Z] Hs-service 0.0.1 Running on 0.0.0.0:8080
```
### 通过服务查询
```
curl "http://127.0.0.1:8080/?q=你叫什么名字"

返回json,表明命中了第一条正则，并返回了正则文件中的附加数据
{
    "Errno": 0,
        "Msg": "",
        "Data": [
        {
            "Id": 1,
            "From": 0,
            "To": 18,
            "Flags": 0,
            "Context": "你叫什么名字",
            "RegexLinev": {
                "Expr": "^[你|叫|什么|的|是]*名字[你|叫|什么|的|是]*$",
                "Data": "{\"type:\"name\", \"user\":\"you\"}"
            }
        }
    ]
}
```
## Bench
比如pttern3.txt中是随机生成了50000个邮箱地址，作为正则表达式,邮箱中的点就表示任意一个字符，所以也算是比较简单的正则了。
```
49991   hwkida@nyveoiwv.net teststring
49992   hjrsiphq@uihseu.net teststring
49993   jeybcfgme@vjomrn.com    teststring
49994   nnthiprbwf@ebpflgne.net teststring
49995   zgsisvddx@mayvf.com teststring
49996   krfwdfwcq@uczmm.net teststring
49997   lzfsw@coikdq.net    teststring
49998   wgaoakpixs@pptizfkr.org teststring
49999   gfthpo@qpxknsku.net teststring
50000   jsxdxlq@cliijbqaqx.org  teststring
```
比如这条结果，就是一个正则匹配的结果.
![](https://ws1.sinaimg.cn/mw690/6973add9gy1fmnedo4l2bj20gf0eijsy.jpg)

以10并发压10w次，得到的结果如下:
![](https://ws1.sinaimg.cn/mw690/6973add9gy1fmnf04tzjfj20mk0ku416.jpg)

可以看到，平均响应时间只有1ms，这还基本都是网络开销，正则查找本身其实只有几十µs。

## 使用
该库使用需要安装hyperscan的类库，安装步骤比较繁琐，可以参考我hyperscan的一个Docker镜像的[Dockerfile](https://hub.docker.com/r/digdeeply/intel-hyperscan-centos7/~/dockerfile/)进行安装。
如果有Docker环境的话，git clone代码后，可以在代码根目录下直接执行`make build`，就会将编译完的二进制放在代码根目录下了。

使用./gohs-ladon -h可以查看帮助文档.
```
$ ./gohs-ladon -h
Gohs-ladon Service 0.0.1

Usage:
gohs-ladon [flags]

Flags:
--debug             Enable debug mode (default true)
    --filepath string   Dict file path
    --flag string       Regex Flag (default "iou")
-h, --help              help for gohs-ladon
    --port int          Listen port (default 8080)
```
