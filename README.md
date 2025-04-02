
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

copy d:\code\feishu2md-main\feishu2md-server.exe d:\code\feishu2md-main\feishu2md_app\assets\backend\

cd d:\code\feishu2md-main\feishu2md_app
flutter build windows --release

构建完成后，您可以在以下目录找到可执行文件： d:\code\feishu2md-main\feishu2md_app\build\windows\runner\Release\


为了方便分发，您可以：

1. 将整个Release目录打包为ZIP文件
2. 使用Inno Setup或NSIS创建安装程序




## TODO 后端服务未正常退出

## 拆分main.dart 文件，便于维护

## 确认程序的app_id 和 app_secret 如何带入的
