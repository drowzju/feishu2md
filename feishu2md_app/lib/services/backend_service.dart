import 'dart:convert';
import 'dart:io';
import 'package:path/path.dart' as path;
import 'package:path_provider/path_provider.dart' as path_provider;

// 全局变量跟踪后端进程
Process? _backendProcess;
bool _backendStarted = false;
bool _isShuttingDown = false;

// 启动后端服务
Future<void> startBackendService() async {
  // 委托给main.dart中的实现
}

// 在应用退出时关闭后端服务
Future<void> stopBackendService() async {
  // 防止重复调用
  if (_isShuttingDown) return;
  _isShuttingDown = true;
  
  if (_backendProcess != null) {
    print('正在关闭后端服务...');
    try {
      // 使用更强力的方式关闭进程
      _backendProcess!.kill(ProcessSignal.sigterm);
      
      // 等待一段时间确保进程有机会关闭
      await Future.delayed(const Duration(seconds: 2));
      
      // 如果进程仍在运行，强制终止
      try {
        if (_backendProcess != null) {
          final int? pid = _backendProcess?.pid;
          if (pid != null && Platform.isWindows) {
            // 在Windows上使用taskkill命令强制终止进程
            await Process.run('taskkill', ['/F', '/PID', pid.toString()]);
            print('使用taskkill强制终止进程: $pid');
          } else {
            _backendProcess!.kill(ProcessSignal.sigkill);
            print('使用sigkill强制终止进程');
          }
        }
      } catch (e) {
        print('强制终止后端进程时出错: $e');
      }
    } catch (e) {
      print('关闭后端服务时出错: $e');
    } finally {
      _backendStarted = false;
      _backendProcess = null;
      _isShuttingDown = false;
      print('后端服务已关闭');
    }
  }
}

// 检查后端服务是否正在运行
Future<bool> isBackendRunning() async {
  if (!_backendStarted || _backendProcess == null) return false;
  
  try {
    // 在Windows上使用tasklist检查进程是否存在
    if (Platform.isWindows) {
      final int? pid = _backendProcess?.pid;
      if (pid != null) {
        final result = await Process.run('tasklist', ['/FI', 'PID eq $pid', '/NH']);
        return result.stdout.toString().contains(pid.toString());
      }
    }
    return false;
  } catch (e) {
    print('检查后端服务状态时出错: $e');
    return false;
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