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
  
  // 添加Windows特定的退出处理
  if (Platform.isWindows) {
    // 注册Windows应用退出处理
    ProcessSignal.sigterm.watch().listen((_) {
      print('收到SIGTERM信号，正在关闭应用...');
      stopBackendService();
    });
    
    // Windows应用可能会收到SIGINT信号
    ProcessSignal.sigint.watch().listen((_) {
      print('收到SIGINT信号，正在关闭应用...');
      stopBackendService();
    });
  }
  
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
    // 只在应用完全分离(detached)时关闭后端服务
    // 避免在hidden或paused状态时关闭服务，因为这些状态在正常操作中也会触发
    if (state == AppLifecycleState.detached) {
      // 应用程序被终止时
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
    
    // 使用 %APPDATA%\feishu2md 作为后端运行目录
    final appDataDir = Platform.environment['APPDATA'] ?? '';
    final backendDir = path.join(appDataDir, 'feishu2md');
    
    // 确保后端目录存在
    await Directory(backendDir).create(recursive: true);
    
    final backendExePath = path.join(appDir.path, 'backend', 'feishu2md-server.exe');
    
    // 检查后端可执行文件是否存在
    final backendFile = File(backendExePath);
    if (!await backendFile.exists()) {
      // 如果不存在，从应用资源中提取
      await _extractBackendExecutable(appDir.path);
    }
    
    // 创建日志目录并确保有写入权限
    final logDir = path.join(backendDir, 'logs');
    final logDirObj = Directory(logDir);
    if (!await logDirObj.exists()) {
      await logDirObj.create(recursive: true);
    }
    
    // 测试日志目录写入权限
    try {
      final testFile = File(path.join(logDir, 'test_write.tmp'));
      await testFile.writeAsString('测试写入权限');
      await testFile.delete();
      print('日志目录写入权限测试通过');
    } catch (e) {
      print('警告: 日志目录写入权限测试失败: $e');
      // 尝试使用备用日志目录
      final tempDir = await path_provider.getTemporaryDirectory();
      final backupLogDir = path.join(tempDir.path, 'feishu2md_logs');
      await Directory(backupLogDir).create(recursive: true);
      print('使用备用日志目录: $backupLogDir');
    }
    
    print('后端工作目录: $backendDir');
    print('日志目录: $logDir');
    
    // 准备环境变量
    final env = Map<String, String>.from(Platform.environment);
    // 确保设置日志相关环境变量
    env['FEISHU2MD_LOG_DIR'] = logDir;
    
    // 启动后端进程，显式指定工作目录和环境变量
    _backendProcess = await Process.start(
      backendExePath,
      ['--port', '8080', '--log-to-file', 'true'],
      workingDirectory: backendDir,
      environment: env,
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
    
    // 检查后端是否成功启动
    try {
      final response = await http.get(Uri.parse('http://localhost:8080/config'));
      if (response.statusCode == 200) {
        print('后端服务已成功启动并响应请求');
      } else {
        print('后端服务启动但响应异常: ${response.statusCode}');
      }
    } catch (e) {
      print('检查后端服务状态时出错: $e');
    }
    
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
    try {
      // 先尝试正常关闭
      _backendProcess!.kill(ProcessSignal.sigterm);
      
      // 等待一段时间确保进程有机会关闭
      bool processExited = false;
      try {
        // 尝试等待进程正常退出
        final exitCode = await _backendProcess!.exitCode.timeout(
          const Duration(seconds: 3),
          onTimeout: () {
            print('等待进程退出超时');
            return -1;
          }
        );
        processExited = true;
        print('后端进程已退出，退出码: $exitCode');
      } catch (e) {
        print('等待进程退出时出错: $e');
      }
      
      // 如果进程仍在运行，强制终止
      if (!processExited && _backendProcess != null) {
        // 在Windows上使用taskkill命令强制终止进程
        if (Platform.isWindows) {
          try {
            final int? pid = _backendProcess?.pid;
            if (pid != null) {
              final result = await Process.run('taskkill', ['/F', '/PID', pid.toString()]);
              print('使用taskkill强制终止进程: $pid, 结果: ${result.exitCode}');
            }
          } catch (e) {
            print('使用taskkill终止进程时出错: $e');
          }
        } else {
          _backendProcess!.kill(ProcessSignal.sigkill);
        }
      }
    } catch (e) {
      print('关闭后端服务时出错: $e');
    } finally {
      _backendStarted = false;
      _backendProcess = null;
      print('后端服务已关闭');
    }
  }
}
