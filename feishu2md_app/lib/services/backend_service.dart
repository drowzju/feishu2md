import 'dart:convert';
import 'dart:io';
import 'package:path/path.dart' as path;
import 'package:path_provider/path_provider.dart' as path_provider;

// 全局变量跟踪后端进程
Process? _backendProcess;
bool _backendStarted = false;

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
    try {
      // 使用更强力的方式关闭进程
      _backendProcess!.kill(ProcessSignal.sigkill);
      
      // 额外检查进程是否已关闭
      final exitCode = await _backendProcess!.exitCode.timeout(
        const Duration(seconds: 3),
        onTimeout: () {
          print('等待进程退出超时，强制终止');
          return -1;
        },
      );
      
      print('后端进程已退出，退出码: $exitCode');
    } catch (e) {
      print('关闭后端服务时出错: $e');
    } finally {
      _backendStarted = false;
      _backendProcess = null;
    }
  }
}