
## 后端服务

cd d:\code\feishu2md-main
go build -o feishu2md-server.exe .\web\
.\feishu2md-server.exe


## 前端服务

cd d:\code\feishu2md-main\feishu2md_app
flutter pub get
flutter run -d windows


## git
git add .
git commit -m "update"
git push -u origin main


## 测试后端
http://localhost:8080/wiki-docs?url=您的飞书知识库URL
http://localhost:8080/wiki-docs?url=https://mxyxpa14jvz.feishu.cn/wiki/KKTBwagWAiUW9ukAl7qcI0CYned


## 拷贝后端并打包编译
把golang的后端服务拷贝到前端的目录下
copy d:\code\feishu2md-main\feishu2md-server.exe d:\code\feishu2md-main\feishu2md_app\assets\backend\

cd d:\code\feishu2md-main\feishu2md_app
flutter build windows --release

构建完成后，您可以在以下目录找到可执行文件： d:\code\feishu2md-main\feishu2md_app\build\windows\runner\Release\


为了方便分发，您可以：

1. 将整个Release目录打包为ZIP文件
2. 使用Inno Setup或NSIS创建安装程序




## 后端服务未正常退出  DONE 
增加了homepage 的dispose

## 拆分main.dart 文件，便于维护 DONE
拆分homepage 和backend service


## 确认程序的app_id 和 app_secret 如何带入的 DONE
保存在本地。
您可以通过以下方式快速访问该文件：

1. 按下 Win + R 打开运行对话框
2. 输入 %APPDATA%\feishu2md 并按回车
3. 这将打开包含 config.json 文件的文件夹

## 问题： 后端会异常退出（待确认） DONE
看起来已经解决。

## 问题： flutter带动后端无日志输出 DONE
看起来也已经解决。

## 问题: 前后端的运行目录不一致导致无法对齐日志文件等信息。 DONE
1. 确保两个程序都运行在同一个目录下。

## 问题： 图片下载失败 DONE

