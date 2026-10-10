# Twintrail

基于 Go 和 myGO v0.3.5 的 macOS 原生双栏文件管理器。完全 Native UI，无 WebView。参考 Marta 的双栏导航与快捷键习惯，支持真实文件操作。

## 已实现

- 目录优先排序、名称升降序、隐藏文件开关、名称子串过滤。
- 复制、移动、重命名、新建多级目录、移动到 macOS 废纸篓，确认对话框与错误显示。
- 文件操作和目录读取在后台执行，过期目录读取结果自动丢弃。
- 复制保留符号链接，拒绝覆盖现有目标，拒绝复制或移动目录到自身内部。跨卷移动目前显示错误，不会自动转为复制删除。
- 系统默认应用打开、Quick Look 预览、Finder 定位、外部 Terminal、原生右键菜单。默认搜索框隐藏，按 ⌘F 或 / 显示。


## 字体偏好设置

应用菜单 **Preference…**（⌘,）提供九项独立字体设置：`actionBar`、`breadcrumbs`、`virtualTabs`、`files`、`statusBar`、`tableHeader`、`tabs`、`preferences`、`terminal`。字体名称可以直接输入，字号范围 8–48 pt。`Default` 使用系统字体（Terminal 使用系统等宽字体），`Default Bold` 与 `Default Light` 使用系统粗体和细体。

当前标签标题使用 `virtualTabs`，其余标签标题使用 `tabs`。修改立即生效，相关行高随字号调整；设置界面自身的字体也能修改。可一键恢复用户给出的默认字体配置。设置自动保存到窗口状态文件的 `fonts` 字段，无须重启或点击“应用”。

## Open With

⌘Return 打开应用选择列表，通过 macOS 原生 NSWorkspace 查询所选文件的可用应用，包含默认应用和“All Applications…”系统选择器。多选时显示所有所选文件共同支持的应用。

应用名称先显示并立即可选，图标在后台渲染为 64×64 位图。应用列表按所选路径缓存一分钟，过期时先显示缓存再后台刷新；图标按应用路径缓存，重复打开不再启动脚本或重复转换图标。

## 内嵌终端

通过 **View → Toggle Terminal**（⌘`）打开或隐藏当前标签页的终端。面板位于窗口最下方，使用 myGO 的 Ghostty 原生 terminal 插件。

每个标签页保留独立的 zsh 登录 shell 和滚动历史；隐藏不会结束会话，关闭标签页或窗口会释放会话。切换标签页或活动栏时，底部显示该标签页的终端状态。

目录导航成功后同步该标签页终端的 CWD。使用独立通知通道同步，不向终端输入流注入 `cd` 命令；正在执行命令时，等待命令结束后再同步，保留已输入但未提交的命令。支持空格、引号等目录名。同步为文件面板到终端的单向同步。

打包时 `go tool mygo build` 自动获取并签名 `libghostty-vt`；开发环境首次运行需要下载该库，也可用 `MYGO_GHOSTTY_VT` 指定已下载的库。

## 开发

```sh
go install github.com/egoist/mygo/cmd/mygo@latest
go tool mygo dev
go tool my vet
```

生成 macOS 应用：

```sh
go tool mygo build --skip-dmg
open build/darwin-arm64/Twintrail.app
```

测试覆盖原生 UI 生命周期、焦点与快捷键、标记、隐藏文件、新建目录、复制移动重命名、防覆盖和递归目标校验，以及终端开关、独立会话、目录同步和关闭清理。
