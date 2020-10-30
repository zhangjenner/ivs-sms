# ivs-sms
Intelligent Video System - Stream Message Service

## 功能描述
-------------
主要负责IPC、NVR、DVR视频流拉取、录像，并提供图像/行为识别的图片数据

## 主要功能特点

- 基于Golang开发维护

- 支持windows、linux平台

- 接收RTSP流推送

- RTSP流分发

- 关键帧缓存

- 秒开画面

- Web后台管理

- 分布式负载均衡


## 安装部署

- 获取代码

        cd $GOPATH/src/github.com
        git clone http://119.3.136.9:3000/Framework/ivs-sms.git ivs-sms
        cd ivs-sms
        
- 直接运行(Windows)

        ivs-sms.exe
    
        以 `Ctrl + C` 停止服务
    
- 以服务启动(Windows)
      
        #先把mlbvc.exe设置成“以管理员身份运行”

        #启动服务
        ./start.bat

        #停止服务
        ./stop.bat


- 直接运行(Linux)

        ./ivs-sms
    
        以 `Ctrl + C` 停止服务

- 以服务启动(Linux)

        #启动服务
	      ./start.sh

        #停止服务
	      ./stop.sh

- 查看界面
	
	打开浏览器输入 [http://localhost:8080](http://localhost:8080), 进入控制页面,默认用户名密码是admin：admin

- 测试推流

	ffmpeg -re -i C:\Users\Administrator\Videos\test.mkv -rtsp_transport tcp -vcodec h264 -f rtsp rtsp://localhost/test

	ffmpeg -re -i C:\Users\Administrator\Videos\test.mkv -rtsp_transport udp -vcodec h264 -f rtsp rtsp://localhost/test
			

- 测试播放

	ffplay -rtsp_transport tcp rtsp://localhost/test

	ffplay rtsp://localhost/test 


## 如何开发

### 开发模式

- 以开发模式运行server (Windows)

        npm run dev:win
        
- 以开发模式运行server (Linux)

        npm run dev:lin

- 以开发模式运行www

        npm run dev:www   

### 编译命令

- 编译server (Windows) 

        npm run build:win

- 编译server (Linux) 

        npm run build:lin                 

- 清理编译文件

        npm run clean 

### 打包文件

        # 打包文件 (windows) 
        npm run build:win
        pack zip

        # 打包文件 (linux)
        npm run build:lin
        pack tar

        # 清理打包文件
        pack clean
