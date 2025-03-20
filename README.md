
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
git push -u origin main


## 测试后端
http://localhost:8080/wiki-docs?url=您的飞书知识库URL
http://localhost:8080/wiki-docs?url=https://mxyxpa14jvz.feishu.cn/wiki/KKTBwagWAiUW9ukAl7qcI0CYned
