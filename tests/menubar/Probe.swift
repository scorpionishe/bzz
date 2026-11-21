import AppKit
import ApplicationServices

// Exercise the WindowServer mouse path; AXPress alone misses pressed highlights.
guard let app = NSRunningApplication.runningApplications(withBundleIdentifier: "com.zlopixatel.bzz").first else { exit(1) }
let process = AXUIElementCreateApplication(app.processIdentifier)
func attribute(_ node: AXUIElement, _ key: String) -> CFTypeRef? {
    var value: CFTypeRef?
    AXUIElementCopyAttributeValue(node, key as CFString, &value)
    return value
}
func children(_ node: AXUIElement) -> [AXUIElement] {
    attribute(node, kAXChildrenAttribute) as? [AXUIElement] ?? []
}
let bar = attribute(process, "AXExtrasMenuBar") as! AXUIElement
guard let item = children(bar).first else { exit(1) }
func rect() -> CGRect {
    var point = CGPoint.zero, size = CGSize.zero
    AXValueGetValue(attribute(item, kAXPositionAttribute) as! AXValue, .cgPoint, &point)
    AXValueGetValue(attribute(item, kAXSizeAttribute) as! AXValue, .cgSize, &size)
    return CGRect(origin: point, size: size)
}
func emit(_ value: Any) {
    FileHandle.standardOutput.write(try! JSONSerialization.data(withJSONObject: value, options: [.sortedKeys]))
}
let inputSource = CGEventSource(stateID: .hidSystemState)!
func mouse(_ type: CGEventType, _ point: CGPoint, flags: CGEventFlags = []) {
    let event = CGEvent(mouseEventSource: inputSource, mouseType: type, mouseCursorPosition: point, mouseButton: .left)!
    event.setIntegerValueField(.mouseEventClickState, value: 1)
    event.flags = flags
    event.post(tap: .cghidEventTap)
}
func key(_ code: CGKeyCode, down: Bool, flags: CGEventFlags = []) {
    let event = CGEvent(keyboardEventSource: inputSource, virtualKey: code, keyDown: down)!
    event.flags = flags
    event.post(tap: .cghidEventTap)
}
func dump(_ node: AXUIElement, _ depth: Int = 0) -> [[String: Any]] {
    guard depth < 9 else { return [] }
    var row: [String: Any] = ["depth": depth]
    for name in [kAXRoleAttribute, kAXTitleAttribute, kAXDescriptionAttribute, kAXValueAttribute] {
        if let value = attribute(node, name) as? String { row[name] = value }
    }
    return [row] + children(node).flatMap { dump($0, depth + 1) }
}
let args = Array(CommandLine.arguments.dropFirst())
var bounds = rect()
var center = CGPoint(x: bounds.midX, y: bounds.midY)
func approach() {
    mouse(.mouseMoved, CGPoint(x: center.x, y: center.y + 50))
    Thread.sleep(forTimeInterval: 0.12)
    // Menu-bar managers and the recording indicator can move items on hover.
    for _ in 0..<15 {
        bounds = rect()
        center = CGPoint(x: bounds.midX, y: bounds.midY)
        mouse(.mouseMoved, center)
        Thread.sleep(forTimeInterval: 0.25)
        if rect() == bounds { return }
    }
    fatalError("Status item did not settle on approach")
}
switch args.first ?? "item" {
case "item":
    emit(["x": bounds.minX, "y": bounds.minY, "width": bounds.width, "height": bounds.height,
          "scale": NSScreen.main!.backingScaleFactor, "screenWidth": NSScreen.main!.frame.width,
          "screenHeight": NSScreen.main!.frame.height])
case "dump": emit(dump(process))
case "axpress":
    // The native menu can enter tracking before AXPress replies (cannotComplete).
    // Callers must verify the visible menu, independently of this return code.
    emit(["AXError": AXUIElementPerformAction(item, kAXPressAction as CFString).rawValue])
case "menu":
    let windows = CGWindowListCopyWindowInfo(.optionOnScreenOnly, 0) as! [[String: Any]]
    emit(["open": windows.contains { $0[kCGWindowOwnerPID as String] as? Int32 == app.processIdentifier &&
          $0[kCGWindowLayer as String] as? Int == Int(CGWindowLevelForKey(.popUpMenuWindow)) }])
case "hold", "click":
    let hold = args.first == "hold" ? 0.4 : 0.06
    approach()
    guard rect() == bounds else { fatalError("Status item moved before mouseDown") }
    let down = ProcessInfo.processInfo.systemUptime
    mouse(.leftMouseDown, center)
    Thread.sleep(forTimeInterval: hold)
    mouse(.leftMouseUp, center)
    emit(["down": down, "up": ProcessInfo.processInfo.systemUptime, "x": center.x, "y": center.y])
case "drag":
    let delta = Double(args[1])!
    approach()
    key(55, down: true, flags: .maskCommand)
    defer { key(55, down: false) }
    mouse(.mouseMoved, center, flags: .maskCommand)
    Thread.sleep(forTimeInterval: 0.15)
    let modifierState = CGEventSource.flagsState(.combinedSessionState).rawValue
    mouse(.leftMouseDown, center, flags: .maskCommand)
    Thread.sleep(forTimeInterval: 0.2)
    for step in 1...30 {
        Thread.sleep(forTimeInterval: 0.02)
        mouse(.leftMouseDragged, CGPoint(x: center.x + delta * Double(step) / 30, y: center.y), flags: .maskCommand)
    }
    for _ in 0..<35 {
        mouse(.leftMouseDragged, CGPoint(x: center.x + delta, y: center.y), flags: .maskCommand)
        Thread.sleep(forTimeInterval: 0.02)
    }
    mouse(.leftMouseUp, CGPoint(x: center.x + delta, y: center.y), flags: .maskCommand)
    key(55, down: false)
    Thread.sleep(forTimeInterval: 0.8)
    emit(["beforeX": bounds.minX, "afterX": rect().minX, "modifierState": modifierState])
default: fatalError("Unknown command")
}
