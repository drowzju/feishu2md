
## 后端服务

cd d:\code\feishu2md-main
go build -o feishu2md-server.exe .\web\server.go
.\feishu2md-server.exe


## 前端服务

cd d:\code\feishu2md-main\feishu2md_app
flutter pub get
flutter run -d windows


## git
git add .
git commit -m "update"
git push -u origin master