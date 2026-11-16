# WLOC — iOS 本机虚拟定位

WLOC 是一个面向 iOS 18+ 的开源本机虚拟定位 App。它通过 Packet Tunnel 在设备内运行本地 MITM 代理，改写 Apple `/clls/wloc` 返回的 Wi‑Fi 与蜂窝基站位置。代理、证书和协议处理均在设备内完成，不依赖远程服务器。

> [!IMPORTANT]
> Packet Tunnel 是受限能力，必须使用付费 Apple Developer 账号签名并在真机运行。App Store / TestFlight 可能拒绝此类应用。

## 已实现

- 全球地图点选、地点搜索、WGS‑84 经纬度输入、收藏和最近位置
- VPN 配置的创建、连接、重连和状态诊断
- 本地 CA 生成、下载与安装引导
- Wi‑Fi 位置以及蜂窝基站字段 22/24 的坐标改写
- 水平/垂直精度、海拔和运动状态改写
- 中国大陆 GCJ-02 / WGS-84 坐标转换
- 境外地址解析失败时自动使用经纬度显示，不影响实际定位

## 工作原理

```text
iOS Location Services
        │  /clls/wloc
        ▼
Packet Tunnel → 本地 Go MITM 代理 → Apple 定位服务
        ▲                  │
        └── 改写 Wi‑Fi / CellTower 坐标、精度和运动状态
```

App 会生成一套仅保存在 App Group 中的本地 CA。安装并信任 CA 后，Tunnel 才能解密目标定位请求并返回修改后的 Protobuf 数据。

## 构建要求

- macOS 与 Xcode 16+
- Go 1.25+
- 付费 Apple Developer 账号；免费账号无法签发 Packet Tunnel entitlement
- 一台 arm64 iPhone；Go 静态库当前不支持模拟器

## 构建

1. 克隆仓库并编译 Go 静态库：

```sh
git clone https://github.com/bennix/WLOC.git
cd WLOC
cd GoSpoofer
./make.sh
```

2. 打开 `WLOC.xcodeproj`。

3. 如果默认标识已被占用，请同时修改以下三项，并在 Swift 配置与 entitlement 文件中保持一致：

   - App Bundle ID：`fudan.miniS.WLOC`
   - Tunnel Bundle ID：`fudan.miniS.WLOC.tunnel`
   - App Group：`group.fudan.miniS.WLOC`

4. 在 `WLOC` 和 `WLOCTunnel` 两个 Target 的 Signing & Capabilities 中选择同一个 Team，并为两个 Target 启用：

   - Network Extensions → Packet Tunnel
   - App Groups → 相同的 App Group

5. 选择连接的 iPhone，运行 `WLOC` Scheme。

如需重新生成工程，可安装 [XcodeGen](https://github.com/yonaskolb/XcodeGen) 后运行：

```sh
xcodegen generate --spec project.yml
```

## 使用

1. 首次启动时允许 App 创建 VPN 配置并连接。
2. 按 App 引导下载 CA，在“设置 → 通用 → VPN 与设备管理”安装。
3. 在“设置 → 通用 → 关于本机 → 证书信任设置”完全信任该 CA。
4. 在地图选择目标位置，或点击 `#` 输入 WGS‑84 经纬度，然后设为虚拟定位。
5. 按提示关闭再打开系统定位服务，然后用地图 App 验证。

## 常见问题

### 为什么不能在模拟器运行？

`GoSpoofer/make.sh` 生成的是 iPhone arm64 静态库，Packet Tunnel 和完整定位链路也需要真机验证。

### 境外搜索没有结果怎么办？

中国区 MapKit 可能无法解析境外 POI。点击搜索栏右侧的 `#`，直接输入 WGS‑84 纬度和经度即可。

### VPN 已连接但位置没有变化？

确认 CA 已安装并开启“完全信任”，然后按 App 提示关闭定位服务、等待数秒并重新开启。诊断信息可在 App 设置页查看。

### 安装后是否应该一直保留 CA？

不使用 WLOC 时建议停止 VPN。卸载 App 后，可同时从系统设置中删除该 CA 和 VPN 配置。

## 致谢与许可证

本项目基于 [acheong08/ios-location-spoofer](https://github.com/acheong08/ios-location-spoofer) 的 iOS/Go 实现，并参考 [mekos2772/ios-location-spoofer](https://github.com/mekos2772/ios-location-spoofer) 对蜂窝基站、多字段及运动状态的处理。

代码遵循 [GNU AGPL-3.0](LICENSE)。仅在你拥有或获准测试的设备与服务上使用。
