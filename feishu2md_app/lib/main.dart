import 'dart:convert';
import 'dart:io';
import 'package:flutter/material.dart';
import 'package:http/http.dart' as http;
import 'package:file_picker/file_picker.dart';
import 'package:path/path.dart' as path;

void main() {
  runApp(const MyApp());
}

class MyApp extends StatelessWidget {
  const MyApp({Key? key}) : super(key: key);

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

class HomePage extends StatefulWidget {
  const HomePage({Key? key}) : super(key: key);

  @override
  _HomePageState createState() => _HomePageState();
}

class _HomePageState extends State<HomePage> {
  final TextEditingController _appIdController = TextEditingController();
  final TextEditingController _appSecretController = TextEditingController();
  final TextEditingController _outputPathController = TextEditingController();
  // 空间地址输入控制器
  final TextEditingController _spaceUrlController = TextEditingController();
  
  bool _isLoading = false;
  String _statusMessage = '';
  double _progress = 0.0; // 新增进度条
  bool _showProgress = false; // 控制进度条显示
  
  // 新增文档树结构
  Map<String, dynamic>? _spaceInfo;
  List<Map<String, dynamic>> _processedNodes = [];
  int _totalNodes = 0;
  int _processedCount = 0;
  
  // 显示当前处理的节点路径
  List<String> _currentPath = [];
  
  // 已处理的节点计数
  int _processedFolders = 0;
  int _processedDocs = 0;
  
  // 新增空间列表相关变量
  List<Map<String, dynamic>> _spacesList = [];
  Map<String, dynamic>? _selectedSpace;
  
  @override
  void initState() {
    super.initState();
    _loadConfig();
  }
  
  // 加载配置
  Future<void> _loadConfig() async {
    setState(() {
      _isLoading = true;
      _statusMessage = '加载配置中...';
    });
    
    try {
      final response = await http.get(Uri.parse('http://localhost:8080/config'));
      
      if (response.statusCode == 200) {
        final data = json.decode(response.body);
        
        setState(() {
          _appIdController.text = data['config']['app_id'] ?? '';
          _appSecretController.text = data['config']['app_secret'] ?? '';
          _outputPathController.text = data['config']['output_path'] ?? '';
        });
        
        setState(() {
          _statusMessage = '配置加载成功';
        });
      }
    } catch (e) {
      setState(() {
        _statusMessage = '加载配置失败: $e';
      });
    } finally {
      setState(() {
        _isLoading = false;
      });
    }
  }
  
  // 保存配置
  Future<void> _saveConfig() async {
    setState(() {
      _isLoading = true;
      _statusMessage = '保存配置中...';
    });
    
    try {
      final response = await http.post(
        Uri.parse('http://localhost:8080/config'),
        headers: {'Content-Type': 'application/json'},
        body: json.encode({
          'app_id': _appIdController.text,
          'app_secret': _appSecretController.text,
          'output_path': _outputPathController.text,
        }),
      );
      
      if (response.statusCode == 200) {
        setState(() {
          _statusMessage = '配置已保存';
        });
      } else {
        setState(() {
          _statusMessage = '保存配置失败: ${response.body}';
        });
      }
    } catch (e) {
      setState(() {
        _statusMessage = '保存配置失败: $e';
      });
    } finally {
      setState(() {
        _isLoading = false;
      });
    }
  }
  
  // 选择输出路径
  Future<void> _selectOutputPath() async {
    String? selectedDirectory = await FilePicker.platform.getDirectoryPath();
    
    if (selectedDirectory != null) {
      setState(() {
        _outputPathController.text = selectedDirectory;
      });
    }
  }
  
  // 下载 _downloadDocument 方法，因为不再需要

  // 使用原子接口获取空间文档
  Future<void> _fetchSpaceDocumentsWithAtomicApis() async {
    // 检查是否有输入或选择的空间
    if (_spaceUrlController.text.isEmpty && _selectedSpace == null) {
      setState(() {
        _statusMessage = '请输入飞书空间地址或从列表中选择空间';
      });
      return;
    }
    
    setState(() {
      _isLoading = true;
      _showProgress = true;
      _progress = 0.0;
      _statusMessage = '正在准备获取空间文档...';
      _processedNodes = [];
      _totalNodes = 0;
      _processedCount = 0;
      _processedFolders = 0;
      _processedDocs = 0;
      _currentPath = [];
    });
    
    try {
      // 先保存配置
      await _saveConfig();
      
      // 根据是否有选定的空间决定流程
      if (_selectedSpace != null) {
        // 已经有选定的空间，直接使用其信息
        _spaceInfo = {
          'space_id': _selectedSpace!['space_id'],
          'space_name': _selectedSpace!['space_name'],
          'node_token': _selectedSpace!['node_token'] ?? '',
        };
        
        setState(() {
          _progress = 0.1;
          _statusMessage = '使用已选空间: ${_spaceInfo!['space_name']}，正在获取顶级节点...';
        });
      } else {
        // 没有选定空间，通过URL获取空间信息
        final encodedUrl = Uri.encodeComponent(_spaceUrlController.text);
        final spaceInfoUrl = 'http://localhost:8080/wiki/space-info?url=$encodedUrl';
        
        setState(() {
          _statusMessage = '正在获取空间信息...';
        });
        
        final spaceInfoResponse = await http.get(Uri.parse(spaceInfoUrl));
        
        if (spaceInfoResponse.statusCode != 200) {
          throw Exception('获取空间信息失败: 服务器返回 ${spaceInfoResponse.statusCode}');
        }
        
        final spaceInfoData = json.decode(spaceInfoResponse.body);
        
        if (spaceInfoData['success'] != true) {
          throw Exception('获取空间信息失败: ${spaceInfoData['message']}');
        }
        
        _spaceInfo = spaceInfoData['space_info'];
        
        setState(() {
          _progress = 0.1;
          _statusMessage = '正在获取顶级节点...';
        });
      }
      
      // 步骤2: 获取顶级节点
      final spaceId = _spaceInfo!['space_id'];
      final topNodesUrl = 'http://localhost:8080/wiki/top-nodes?space_id=$spaceId';
      
      final topNodesResponse = await http.get(Uri.parse(topNodesUrl));
      
      if (topNodesResponse.statusCode != 200) {
        throw Exception('获取顶级节点失败: 服务器返回 ${topNodesResponse.statusCode}');
      }
      
      final topNodesData = json.decode(topNodesResponse.body);
      
      if (topNodesData['success'] != true) {
        throw Exception('获取顶级节点失败: ${topNodesData['message']}');
      }
      
      final topNodes = topNodesData['nodes'] as List;
      
      // 计算预估的总节点数（初始估计）
      setState(() {
        _progress = 0.2;
        _statusMessage = '开始构建文档树...';
        _totalNodes = topNodes.length * 5; // 初始估计每个顶级节点平均有5个子节点
      });
      
      // 步骤3: 递归获取所有节点
      final rootNode = {
        'title': _spaceInfo!['space_name'],
        'url': 'https://feishu.cn/wiki/${_spaceInfo!['node_token']}',
        'type': 'space',
        'children': [],
      };
      
      _currentPath.add(_spaceInfo!['space_name']);
      
      // 递归处理所有节点
      for (var i = 0; i < topNodes.length; i++) {
        var node = Map<String, dynamic>.from(topNodes[i]);
        print('处理顶级节点 ${i+1}/${topNodes.length}: ${node['title']}');
        await _processNode(node, rootNode['children'] as List);
      }
      
      setState(() {
        _progress = 0.9;
        _statusMessage = '正在保存文档树...';
      });
      
      // 步骤4: 保存文档树
      final saveTreeUrl = 'http://localhost:8080/wiki/save-tree';
      final saveTreeResponse = await http.post(
        Uri.parse(saveTreeUrl),
        headers: {'Content-Type': 'application/json'},
        body: json.encode({
          'output_path': _outputPathController.text,
          'space_name': _spaceInfo!['space_name'],
          'tree': rootNode,
        }),
      );
      
      if (saveTreeResponse.statusCode != 200) {
        throw Exception('保存文档树失败: 服务器返回 ${saveTreeResponse.statusCode}');
      }
      
      final saveTreeData = json.decode(saveTreeResponse.body);
      
      if (saveTreeData['success'] != true) {
        throw Exception('保存文档树失败: ${saveTreeData['message']}');
      }
      
      setState(() {
        _progress = 1.0;
        _statusMessage = '文档树已生成到: ${saveTreeData['file_path']}\n共处理 $_processedFolders 个文件夹, $_processedDocs 个文档';
      });
    } catch (e) {
      setState(() {
        _statusMessage = '获取文档失败: $e';
      });
    } finally {
      setState(() {
        _isLoading = false;
        // 保持进度条显示一段时间，然后隐藏
        Future.delayed(const Duration(seconds: 5), () {
          if (mounted) {
            setState(() {
              _showProgress = false;
            });
          }
        });
      });
    }
  }
  
  // 修改：递归处理节点 - 对所有节点尝试获取子节点
  Future<void> _processNode(Map<String, dynamic> node, List children) async {
    // 更新当前路径
    _currentPath.add(node['title']);
    
    // 添加当前节点到子节点列表
    final nodeData = {
      'title': node['title'],
      'url': node['url'],
      'type': node['type'],
      'children': [],
    };
    
    children.add(nodeData);
    
    // 打印节点详细信息，帮助调试
    print('节点详情: ${json.encode(node)}');
    
    // 确保有node_token
    final nodeToken = node['node_token'];
    if (nodeToken == null || nodeToken.isEmpty) {
      print('节点 ${node['title']} 没有node_token，跳过获取子节点');
      
      // 更新处理计数 - 没有node_token的视为文档
      setState(() {
        _processedCount++;
        _processedDocs++;
        
        // 更新进度和状态消息
        _progress = 0.2 + (_processedCount / _totalNodes) * 0.7;
        if (_progress > 0.9) _progress = 0.9; // 限制最大进度
        
        // 显示当前处理路径
        _statusMessage = '正在处理: ${_currentPath.join(" > ")}\n'
            '已处理: $_processedCount 个节点 (文件夹: $_processedFolders, 文档: $_processedDocs)';
      });
      
      _currentPath.removeLast();
      return;
    }
    
    final childrenUrl = 'http://localhost:8080/wiki/node-children?node_token=$nodeToken';
    
    try {
      print('尝试获取节点 ${node['title']} 的子节点，URL: $childrenUrl');
      final childrenResponse = await http.get(Uri.parse(childrenUrl));
      
      if (childrenResponse.statusCode != 200) {
        print('获取子节点失败: 服务器返回 ${childrenResponse.statusCode}，认为节点 ${node['title']} 没有子节点');
        
        // 更新处理计数 - 获取子节点失败的视为文档
        setState(() {
          _processedCount++;
          _processedDocs++;
          
          // 更新进度和状态消息
          _progress = 0.2 + (_processedCount / _totalNodes) * 0.7;
          if (_progress > 0.9) _progress = 0.9; // 限制最大进度
          
          // 显示当前处理路径
          _statusMessage = '正在处理: ${_currentPath.join(" > ")}\n'
              '已处理: $_processedCount 个节点 (文件夹: $_processedFolders, 文档: $_processedDocs)';
        });
        
        _currentPath.removeLast();
        return;
      }
      
      final childrenData = json.decode(childrenResponse.body);
      
      if (childrenData['success'] != true) {
        print('获取子节点失败: ${childrenData['message']}，认为节点 ${node['title']} 没有子节点');
        
        // 更新处理计数 - 获取子节点失败的视为文档
        setState(() {
          _processedCount++;
          _processedDocs++;
          
          // 更新进度和状态消息
          _progress = 0.2 + (_processedCount / _totalNodes) * 0.7;
          if (_progress > 0.9) _progress = 0.9; // 限制最大进度
          
          // 显示当前处理路径
          _statusMessage = '正在处理: ${_currentPath.join(" > ")}\n'
              '已处理: $_processedCount 个节点 (文件夹: $_processedFolders, 文档: $_processedDocs)';
        });
        
        _currentPath.removeLast();
        return;
      }
      
      final childNodes = childrenData['nodes'] as List;
      print('节点 ${node['title']} 有 ${childNodes.length} 个子节点');
      
      // 更新处理计数 - 有子节点的才视为文件夹
      setState(() {
        _processedCount++;
        
        // 只有成功获取到子节点且子节点数量大于0的才算作文件夹
        if (childNodes.isNotEmpty) {
          _processedFolders++;
        } else {
          _processedDocs++;
        }
        
        // 更新进度和状态消息
        _progress = 0.2 + (_processedCount / _totalNodes) * 0.7;
        if (_progress > 0.9) _progress = 0.9; // 限制最大进度
        
        // 显示当前处理路径
        _statusMessage = '正在处理: ${_currentPath.join(" > ")}\n'
            '已处理: $_processedCount 个节点 (文件夹: $_processedFolders, 文档: $_processedDocs)';
      });
      
      if (childNodes.isEmpty) {
        print('节点 ${node['title']} 没有子节点');
        _currentPath.removeLast();
        return;
      }
      
      // 更新总节点数估计
      setState(() {
        _totalNodes += childNodes.length;
      });
      
      // 递归处理子节点
      for (var i = 0; i < childNodes.length; i++) {
        var childNode = Map<String, dynamic>.from(childNodes[i]);
        print('处理子节点 ${i+1}/${childNodes.length}: ${childNode['title']}');
        await _processNode(childNode, nodeData['children'] as List);
      }
    } catch (e) {
      print('处理节点 ${node['title']} 的子节点时出错: $e，继续处理其他节点');
      
      // 更新处理计数 - 处理出错的视为文档
      setState(() {
        _processedCount++;
        _processedDocs++;
        
        // 更新进度和状态消息
        _progress = 0.2 + (_processedCount / _totalNodes) * 0.7;
        if (_progress > 0.9) _progress = 0.9; // 限制最大进度
        
        // 显示当前处理路径
        _statusMessage = '正在处理: ${_currentPath.join(" > ")}\n'
            '已处理: $_processedCount 个节点 (文件夹: $_processedFolders, 文档: $_processedDocs)';
      });
    }
    
    // 处理完成，从路径中移除当前节点
    _currentPath.removeLast();
  }

  // 添加获取所有空间列表的方法
  Future<void> _fetchAllWikiSpaces() async {
    setState(() {
      _isLoading = true;
      _statusMessage = '正在获取空间列表...';
    });
    
    try {
      // 先保存配置
      await _saveConfig();
      
      final response = await http.get(Uri.parse('http://localhost:8080/wiki/spaces'));
      
      if (response.statusCode != 200) {
        throw Exception('获取空间列表失败: 服务器返回 ${response.statusCode}');
      }
      
      final data = json.decode(response.body);
      
      if (data['success'] != true) {
        throw Exception('获取空间列表失败: ${data['message']}');
      }
      
      setState(() {
        _spacesList = List<Map<String, dynamic>>.from(data['spaces']);
        _statusMessage = '获取到 ${_spacesList.length} 个空间';
        
        // 如果有空间，默认选择第一个
        if (_spacesList.isNotEmpty) {
          _selectedSpace = _spacesList[0];
          _spaceUrlController.text = _selectedSpace!['url'];
        }
      });
    } catch (e) {
      setState(() {
        _statusMessage = '获取空间列表失败: $e';
      });
    } finally {
      setState(() {
        _isLoading = false;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('飞书文档下载器'),
      ),
      body: Padding(
        padding: const EdgeInsets.all(16.0),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            // 获取空间列表按钮
            ElevatedButton(
              onPressed: _isLoading ? null : _fetchAllWikiSpaces,
              child: const Text('获取所有空间列表'),
              style: ElevatedButton.styleFrom(
                padding: const EdgeInsets.symmetric(vertical: 16),
                backgroundColor: Colors.blue,
              ),
            ),
            
            const SizedBox(height: 16),
            
            // 空间选择下拉列表
            if (_spacesList.isNotEmpty)
              Container(
                padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 4),
                decoration: BoxDecoration(
                  border: Border.all(color: Colors.grey),
                  borderRadius: BorderRadius.circular(4),
                ),
                child: DropdownButton<Map<String, dynamic>>(
                  isExpanded: true,
                  underline: Container(), // 移除下划线
                  value: _selectedSpace,
                  hint: const Text('选择空间'),
                  onChanged: (Map<String, dynamic>? newValue) {
                    setState(() {
                      _selectedSpace = newValue;
                      if (newValue != null) {
                        _spaceUrlController.text = newValue['url'];
                      }
                    });
                  },
                  items: _spacesList.map<DropdownMenuItem<Map<String, dynamic>>>((Map<String, dynamic> space) {
                    return DropdownMenuItem<Map<String, dynamic>>(
                      value: space,
                      child: Text(space['space_name']),
                    );
                  }).toList(),
                ),
              ),
            
            const SizedBox(height: 16),
            
            // 空间地址输入框
            TextField(
              controller: _spaceUrlController,
              decoration: const InputDecoration(
                labelText: '飞书空间地址',
                hintText: '输入飞书空间地址，用于递归获取所有文档',
                border: OutlineInputBorder(),
              ),
            ),
            
            const SizedBox(height: 16),
            
            // 获取空间文档按钮
            ElevatedButton(
              onPressed: _isLoading ? null : _fetchSpaceDocumentsWithAtomicApis,
              child: const Text('获取空间所有文档'),
              style: ElevatedButton.styleFrom(
                padding: const EdgeInsets.symmetric(vertical: 16),
                backgroundColor: Colors.green,
              ),
            ),
            
            const SizedBox(height: 16),
            // 显示进度条
            if (_showProgress)
              Column(
                children: [
                  LinearProgressIndicator(value: _progress),
                  const SizedBox(height: 8),
                  Text('进度: ${(_progress * 100).toStringAsFixed(1)}%'),
                  const SizedBox(height: 4),
                  Container(
                    padding: const EdgeInsets.all(8),
                    decoration: BoxDecoration(
                      border: Border.all(color: Colors.grey.shade300),
                      borderRadius: BorderRadius.circular(4),
                    ),
                    child: Text(
                      _statusMessage,
                      style: const TextStyle(fontSize: 12),
                    ),
                  ),
                ],
              ),
            const SizedBox(height: 8),
            ExpansionTile(
              title: const Text('配置'),
              children: [
                TextField(
                  controller: _appIdController,
                  decoration: const InputDecoration(
                    labelText: 'App ID',
                    hintText: '输入飞书App ID',
                    border: OutlineInputBorder(),
                  ),
                ),
                const SizedBox(height: 8),
                TextField(
                  controller: _appSecretController,
                  decoration: const InputDecoration(
                    labelText: 'App Secret',
                    hintText: '输入飞书App Secret',
                    border: OutlineInputBorder(),
                  ),
                  obscureText: true,
                ),
                const SizedBox(height: 8),
                Row(
                  children: [
                    Expanded(
                      child: TextField(
                        controller: _outputPathController,
                        decoration: const InputDecoration(
                          labelText: '输出路径',
                          hintText: '选择文档保存位置',
                          border: OutlineInputBorder(),
                        ),
                        readOnly: true,
                      ),
                    ),
                    IconButton(
                      icon: const Icon(Icons.folder_open),
                      onPressed: _selectOutputPath,
                    ),
                  ],
                ),
                const SizedBox(height: 8),
                ElevatedButton(
                  onPressed: _saveConfig,
                  child: const Text('保存配置'),
                ),
              ],
            ),
            const SizedBox(height: 16),
            if (_isLoading && !_showProgress)
              const Center(child: CircularProgressIndicator())
            else if (_statusMessage.isNotEmpty)
              Text(
                _statusMessage,
                style: TextStyle(
                  color: _statusMessage.contains('失败') ? Colors.red : Colors.green,
                ),
                textAlign: TextAlign.center,
              ),
          ],
        ),
      ),
    );
  }
}
