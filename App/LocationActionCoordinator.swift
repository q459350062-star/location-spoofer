
import Foundation

@MainActor
protocol LocationActionProxying: AnyObject {
    var isRunning: Bool { get }
    func start() async throws
    func stop()
    func setCoords(lat: Double, lon: Double, enabled: Bool, accuracy: Int) -> UInt64
}

extension ProxyManager: LocationActionProxying {}

@MainActor
protocol LocationActionSettingsStoring: AnyObject {
    func load() -> WlocSettings?
    func save(_ settings: WlocSettings)
    func clear()
}

@MainActor
final class DeviceWlocSettingsStorage: LocationActionSettingsStoring {
    func load() -> WlocSettings? {
        WlocSettingsStore.load()
    }

    func save(_ settings: WlocSettings) {
        WlocSettingsStore.save(settings)
    }

    func clear() {
        WlocSettingsStore.clear()
    }
}

@MainActor
final class LocationActionCoordinator: ObservableObject {
    @Published private(set) var state: LocationActionState = .idle
    @Published private(set) var virtualLocationEnabled = false
    @Published private(set) var message = ""

    private let proxy: any LocationActionProxying
    private let settings: any LocationActionSettingsStoring

    init() {
        self.proxy = ProxyManager.shared
        self.settings = DeviceWlocSettingsStorage()
        self.virtualLocationEnabled = settings.load()?.enabled == true
    }

    init(
        proxy: any LocationActionProxying,
        settings: any LocationActionSettingsStoring
    ) {
        self.proxy = proxy
        self.settings = settings
        self.virtualLocationEnabled = settings.load()?.enabled == true
    }

    func apply(_ favorite: FavoriteLocation) async -> Bool {
        guard beginApply() else { return false }

        let wasRunning = proxy.isRunning

        do {
            if !wasRunning {
                try await proxy.start()
            }

            guard !Task.isCancelled else {
                if !wasRunning {
                    proxy.stop()
                }
                finishCancelledApply()
                return false
            }

            let applied = commit(favorite)
            if !applied && !wasRunning {
                proxy.stop()
            }
            return applied

        } catch {
            if !wasRunning {
                proxy.stop()
            }

            failApply(error)
            return false
        }
    }

    /// 提交已经完成验证的位置
    func applyVerified(_ favorite: FavoriteLocation) -> Bool {
        guard beginApply() else { return false }

        guard proxy.isRunning else {
            failApply(ProxyError.startFailed)
            return false
        }

        return commit(favorite)
    }

    /// 关闭虚拟定位，同时停止本地代理
    func clear() {
        guard !state.isBusy else { return }

        // 先发送关闭定位指令
        _ = proxy.setCoords(
            lat: 0,
            lon: 0,
            enabled: false,
            accuracy: 25
        )

        // 清除持久化配置
        settings.clear()

        // 停止代理
        proxy.stop()

        RuntimeLogger.info(
            "APP",
            "Location",
            "虚拟定位已关闭，本地代理已停止"
        )

        state = .idle
        virtualLocationEnabled = false
        message = "已恢复真实定位"
    }

    private func beginApply() -> Bool {
        guard !state.isBusy else { return false }

        state = .applyingLocation
        message = "启动代理…"

        return true
    }

    private func commit(_ favorite: FavoriteLocation) -> Bool {
        // 固定使用 WGS-84 坐标；在写入代理前拒绝非法或非有限坐标。
        let wgs = favorite.coordinatePair.wgs84
        guard wgs.latitude.isFinite,
              wgs.longitude.isFinite,
              (-90.0...90.0).contains(wgs.latitude),
              (-180.0...180.0).contains(wgs.longitude) else {
            failApply(LocationActionError.invalidCoordinate)
            return false
        }

        let value = WlocSettings(
            longitude: wgs.longitude,
            latitude: wgs.latitude,
            accuracy: favorite.accuracy,
            enabled: true
        )

        settings.save(value)

        _ = proxy.setCoords(
            lat: wgs.latitude,
            lon: wgs.longitude,
            enabled: true,
            accuracy: favorite.accuracy
        )

        RuntimeLogger.info(
            "APP",
            "坐标转换",
            "设置虚拟定位坐标",
            details: [
                "WLOC写入标准":
                    CoordinateConverter.MapCoordinateSystem.wgs84.diagnosticName,
                "当前地图标准":
                    CoordinateConverter.currentMapCoordinateSystem.diagnosticName,
                "目标所在区域":
                    CoordinateConverter.usesGCJ02ServiceArea(
                        lat: wgs.latitude,
                        lon: wgs.longitude
                    ) ? "国内转换区域" : "国外非转换区域",
                "取值字段": "coordinatePair.wgs84",
                "accuracy": String(favorite.accuracy)
            ]
        )

        state = .idle
        virtualLocationEnabled = true
        message = "虚拟定位已开启"

        return true
    }

    private func finishCancelledApply() {
        state = .idle
        message = "已取消位置更新"
    }

    private func failApply(_ error: Error) {
        virtualLocationEnabled = false
        message = "启动失败"
        state = .failed(error.localizedDescription)

        RuntimeLogger.error(
            "APP",
            "Location",
            "apply失败",
            error: error
        )
    }
}


private enum LocationActionError: LocalizedError {
    case invalidCoordinate

    var errorDescription: String? {
        switch self {
        case .invalidCoordinate:
            return "坐标无效：纬度必须在 -90 至 90、经度必须在 -180 至 180 之间"
        }
    }
}
