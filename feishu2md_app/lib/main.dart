import 'dart:convert';
import 'dart:io';
import 'package:flutter/material.dart';
import 'package:http/http.dart' as http;
import 'package:file_picker/file_picker.dart';
import 'package:path/path.dart' as path;
import 'package:path_provider/path_provider.dart' as path_provider;
import 'pages/home_page.dart';
import 'services/backend_service.dart';

// 添加全局变量跟踪后端进程
Process? _backendProcess;
bool _backendStarted = false;

void main() async {
  // 确保Flutter绑定初始化
  WidgetsFlutterBinding.ensureInitialized();
  
  // 启动后端服务
  await startBackendService();
  
  runApp(const MyApp());
}

class MyApp extends StatefulWidget {
  const MyApp({Key? key}) : super(key: key);

  @override
  _MyAppState createState() => _MyAppState();
}

class _MyAppState extends State<MyApp> with WidgetsBindingObserver {
  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    // 确保在应用退出时关闭后端服务
    stopBackendService();
    super.dispose();
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    print('应用生命周期状态变化: $state');
    // 监听多种可能导致应用退出的状态
    if (state == AppLifecycleState.detached || 
        state == AppLifecycleState.paused || 
        state == AppLifecycleState.inactive) {
      // 应用程序可能被终止
      stopBackendService();
    }
  }

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: '飞书文档下载器',
      theme: ThemeData(
        primarySwatch: Colors.blue,
        visualDensity: VisualDensity.adaptivePlatformDensity,
      ),
      home: const HomePage(),
    );
  }
}

// 启动后端服务的函数
Future<void> startBackendService() async {
  if (_backendStarted) return;
  
  try {
    // 获取应用程序目录
    final appDir = await path_provider.getApplicationSupportDirectory();
    final backendExePath = path.join(appDir.path, 'backend', 'feishu2md-server.exe');
    
    // 检查后端可执行文件是否存在
    final backendFile = File(backendExePath);
    if (!await backendFile.exists()) {
      // 如果不存在，从应用资源中提取
      await _extractBackendExecutable(appDir.path);
    }
    
    // 启动后端进程
    _backendProcess = await Process.start(
      backendExePath,
      ['--port', '8080'],
      workingDirectory: path.join(appDir.path, 'backend'),
    );
    
    _backendStarted = true;
    
    // 监听后端输出（可选，用于调试）
    _backendProcess!.stdout.transform(utf8.decoder).listen((data) {
      print('后端输出: $data');
    });
    
    _backendProcess!.stderr.transform(utf8.decoder).listen((data) {
      print('后端错误: $data');
    });
    
    // 等待后端启动
    await Future.delayed(const Duration(seconds: 2));
    
    print('后端服务已启动');
  } catch (e) {
    print('启动后端服务失败: $e');
  }
}

// 从应用资源中提取后端可执行文件
Future<void> _extractBackendExecutable(String appDirPath) async {
  try {
    // 创建后端目录
    final backendDir = Directory(path.join(appDirPath, 'backend'));
    if (!await backendDir.exists()) {
      await backendDir.create(recursive: true);
    }
    
    // 这里需要根据实际情况从Flutter资源中提取后端可执行文件
    // 在开发阶段，可以手动复制
    final sourceBackendExe = File('d:\\code\\feishu2md-main\\feishu2md-server.exe');
    if (await sourceBackendExe.exists()) {
      await sourceBackendExe.copy(path.join(backendDir.path, 'feishu2md-server.exe'));
      print('后端可执行文件已复制到: ${backendDir.path}');
    } else {
      throw Exception('源后端可执行文件不存在');
    }
  } catch (e) {
    print('提取后端可执行文件失败: $e');
    rethrow;
  }
}

// 在应用退出时关闭后端服务
Future<void> stopBackendService() async {
  if (_backendProcess != null) {
    print('正在关闭后端服务...');
    _backendProcess!.kill();
    _backendStarted = false;
    _backendProcess = null;
  }
}
