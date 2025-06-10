# software-engineer


## 项目介绍


## 技术栈


## 项目架构


## 项目部署
### 前端部署
1. **安装依赖**
```bash
    cd frontend/vue-project
    npm install
```

2. **启动项目**
```bash
    cd frontend/vue-project
    npm run dev
```

### 后端部署
1. **安装依赖**
```bash
    cd backend/server
    go mod tidy
```
如果有拉取不完全的，可以使用 `go get + 依赖` 手动拉取。


2. **环境配置·**
在backend/server/config/configs目录下创建一个config.yaml文件（或修改提供的config.yaml.example文件）添加以下内容：
```yaml
db: # 数据库配置
  host: localhost # 数据库地址
  port: "5432"   # 数据库端口
  name: database # 数据库名
  user: username    # 数据库用户名
  password: password # 数据库密码

oss: #aliyun oss 配置
  OSS_REGION: oss-cn-shenzhen # oss区域
  OSS_ACCESS_KEY_ID: # oss key
  OSS_ACCESS_KEY_SECRET:  # oss密钥
  OSS_BUCKET:  # oss bucket

email:
    email_name: xxxx@163.com
    email_password: # 应用密码
    #（在163邮箱 -> 设置 -> POP3/SMTP/IMAP -> 开启服务 -> 开启IMAP/SMTP服务. POP3/SMTP服务 -> 保存开启后弹出窗口显示的应用密码（随后消失不可查看））
    base_url: # ngrok提供的url
    # 密码找回的方式二是发送验证码，可以不用暴露内网，不使用这个base_url
    # 使用ngrok暴露内网（测试环境）
    # ngrok http --url=ox-driven-shortly.ngrok-free.app 80
    # ngrok http http://localhost:8080
    # 安装ngrok:https://dashboard.ngrok.com/get-started/setup/windows

smtp_server:
  SMTPServer_host: smtp.163.com
  SMTPServer_port: 25
```

3. **启动**
```bash
    cd backend/server
    go run main.go
```

## 其他说明
<!-- 1. **密码找回功能接口测试**
启动ngork服务，通过ngrok http 暴露本地8080端口，并在backend/server/config/configs中配置base_url；
访问登录localhost:8080/agent/login，获取token；然后在请求头中添加token，访问localhost:8080/agent/request_reset_password，参数为
```json
{
    "email":"xxx@xxx.com"  // 你注册时使用的邮箱
}
```
然后会收到邮箱，点击邮件上的链接跳转至密码找回页面，输入新密码，点击确定，即可更新密码 -->







## 用户故事讨论：
一. 前端：








二. 后端：
1. 通过监控服务器的CPU、内存、磁盘、网卡等关键指标，一旦发现资源异常波动，即可及时了解并通知运维人员。
监控系统可以触发预警提醒，通过短信、邮件、声音、脚本等多种通讯工具通知运维人员，确保他们能够及时响应并处理问题，避免业务中断。

2. 远程访问与管理：
允许管理员通过Web界面或命令行界面远程访问服务器，进行配置、更新和维护操作。

3. 安全审计：
记录和审计管理员对服务器的所有操作，包括登录、配置更改、文件操作等，确保服务器的安全性。

4. 密码找回











要实现一个页面左右两部分分别显示两个系统的文件系统，并通过拖放方式实现文件传输，您可以采用以下方案：

### 实现思路

1. **文件系统展示**：
   - 使用 `Vue` 组件分别展示左右两个系统的文件系统结构，可以使用树形控件（如 `Element UI` 的 `el-tree` 或 `Vue-Tree`）来呈现文件和文件夹。
   - 通过 `Gin` 后端提供 API 接口，获取两个系统的文件列表，并在前端动态渲染。

2. **拖放功能**：
   - 利用 `Vue` 的拖放事件（`@dragstart`、`@dragover`、`@drop`）实现文件拖放功能。
   - 当用户拖动文件时，记录拖动的文件信息（如文件路径）。
   - 在目标区域释放文件时，触发文件传输操作。

3. **文件传输**：
   - 通过 `Gin` 后端编写文件传输的 API 接口，实现从一个系统复制或移动文件到另一个系统。
   - 前端在 `@drop` 事件中调用后端的文件传输接口，完成文件传输。

### 现成组件推荐

目前没有完全满足您需求的现成 `Vue` 组件，但可以结合以下组件实现：

- **文件系统展示**：
  - **Element UI 的 `el-tree`**：用于展示文件系统结构，支持拖放功能。
  - **Vue-Tree**：轻量级的树形组件，易于定制。

- **拖放功能**：
  - **Vue.Draggable**：提供拖放排序功能，可用于文件拖放。
  - **Vue-Drag-and-Drop**：简单的拖放组件，易于集成。

### 实现步骤

1. **安装依赖**：
   - 安装 `Element UI` 或 `Vue-Tree` 用于文件系统展示。
   - 安装 `Vue.Draggable` 或 `Vue-Drag-and-Drop` 用于拖放功能。

2. **编写文件系统展示组件**：
   - 创建两个组件分别展示左右两个系统的文件系统。
   - 使用树形组件展示文件列表，并通过 `Gin` 后端 API 获取数据。

3. **实现拖放功能**：
   - 在文件树组件中添加拖放事件监听。
   - 在 `@dragstart` 事件中记录拖动的文件信息。
   - 在 `@drop` 事件中调用 `Gin` 后端的文件传输接口。

4. **编写 Gin 后端文件传输接口**：
   - 实现文件复制或移动的逻辑，从一个系统传输到另一个系统。
   - 提供对应的 API 接口供前端调用。

### 示例代码

#### 前端（Vue）

```vue
<template>
  <div class="file-transfer">
    <div class="left-panel">
      <el-tree
        :data="leftFileSystem"
        :props="treeProps"
        @drag-start="handleDragStart"
        @drop="handleDrop"
      ></el-tree>
    </div>
    <div class="right-panel">
      <el-tree
        :data="rightFileSystem"
        :props="treeProps"
        @drag-start="handleDragStart"
        @drop="handleDrop"
      ></el-tree>
    </div>
  </div>
</template>

<script>
import { getLeftFiles, getRightFiles } from '@/api/filesystem';

export default {
  data() {
    return {
      leftFileSystem: [],
      rightFileSystem: [],
      treeProps: {
        label: 'name',
        children: 'children',
      },
      draggedFilePath: '',
    };
  },
  mounted() {
    this.loadFileSystemData();
  },
  methods: {
    async loadFileSystemData() {
      this.leftFileSystem = await getLeftFiles();
      this.rightFileSystem = await getRightFiles();
    },
    handleDragStart(event, treeNode) {
      this.draggedFilePath = treeNode.filePath;
    },
    async handleDrop(event, treeNode) {
      const targetPath = treeNode.filePath;
      await this.transferFile(this.draggedFilePath, targetPath);
      this.loadFileSystemData(); // 刷新文件系统数据
    },
    async transferFile(sourcePath, targetPath) {
      // 调用 Gin 后端文件传输接口
      const response = await axios.post('/api/transfer', {
        sourcePath,
        targetPath,
      });
      if (response.status === 200) {
        console.log('文件传输成功');
      } else {
        console.error('文件传输失败');
      }
    },
  },
};
</script>

<style scoped>
.file-transfer {
  display: flex;
}
.left-panel,
.right-panel {
  width: 50%;
}
</style>
```

#### 后端（Gin）

```go
package main

import (
"fmt"
"net/http"
"os"

"github.com/gin-gonic/gin"
)

func main() {
r := gin.Default()

// 获取左侧系统文件列表
r.GET("/api/left/files", func(c *gin.Context) {
// 实现获取左侧文件系统数据的逻辑
// ...
c.JSON(http.StatusOK, leftFileSystemData)
})

// 获取右侧系统文件列表
r.GET("/api/right/files", func(c *gin.Context) {
// 实现获取右侧文件系统数据的逻辑
// ...
c.JSON(http.StatusOK, rightFileSystemData)
})

// 文件传输接口
r.POST("/api/transfer", func(c *gin.Context) {
sourcePath := c.PostForm("sourcePath")
targetPath := c.PostForm("targetPath")
err := transferFile(sourcePath, targetPath)
if err != nil {
c.String(http.StatusInternalServerError, fmt.Sprintf("文件传输失败: %s", err.Error()))
} else {
c.String(http.StatusOK, "文件传输成功")
}
})

r.Run(":8080")
}

// 文件传输函数
func transferFile(sourcePath, targetPath string) error {
// 实现文件从 sourcePath 传输到 targetPath 的逻辑
// 可以使用 os.Copy() 或自定义复制逻辑
// ...
return nil
}
```

### 注意事项

- 确保前后端数据格式一致，方便前端组件渲染。
- 处理文件传输过程中的异常情况，如文件不存在、权限问题等。
- 考虑文件传输的安全性，必要时对传输的文件进行加密或验证。

通过以上方案，您可以实现一个页面左右两部分显示两个系统的文件系统，并通过拖放方式实现文件传输的功能。