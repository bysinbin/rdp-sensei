import AppKit
import Foundation

// ========================================================
// Rdp Sensei - Physical Touch Bar Native Agent for macOS
// Zero external dependencies. Uses standard macOS AppKit.
// Auto-dismisses Touch Bar when user switches to other apps.
// Only presents when RDP session is active AND app is in focus.
// ========================================================

final class TouchBarAgent: NSObject, NSApplicationDelegate, NSTouchBarDelegate {
    static let shared = TouchBarAgent()
    
    var touchBar: NSTouchBar?
    var serverPort: Int = 8855
    var isPresented: Bool = false
    var isSessionActive: Bool = false
    var isAppFrontmost: Bool = true
    
    struct KeyItem {
        let id: String
        let title: String
        let keyName: String
        let width: CGFloat
        let category: String // "danger", "accent", "nav", "fn", "close"
    }
    
    let keys: [KeyItem] = [
        // Primary Essential Controls
        KeyItem(id: "esc", title: "Esc", keyName: "Escape", width: 44, category: "danger"),
        KeyItem(id: "del", title: "Del", keyName: "Delete", width: 42, category: "danger"),
        KeyItem(id: "home", title: "Home", keyName: "Home", width: 44, category: "nav"),
        KeyItem(id: "end", title: "End", keyName: "End", width: 40, category: "nav"),
        KeyItem(id: "pgup", title: "PgUp", keyName: "PageUp", width: 44, category: "nav"),
        KeyItem(id: "pgdn", title: "PgDn", keyName: "PageDown", width: 44, category: "nav"),
        KeyItem(id: "ins", title: "Ins", keyName: "Insert", width: 38, category: "nav"),
        KeyItem(id: "prtsc", title: "PrtSc", keyName: "PrintScreen", width: 44, category: "nav"),
        KeyItem(id: "alttab", title: "⇥ Alt+Tab", keyName: "AltTab", width: 68, category: "accent"),
        KeyItem(id: "win", title: "⊞ Win", keyName: "Meta", width: 48, category: "accent"),

        // Function Keys F1-F12
        KeyItem(id: "f1", title: "F1", keyName: "F1", width: 36, category: "fn"),
        KeyItem(id: "f2", title: "F2", keyName: "F2", width: 36, category: "fn"),
        KeyItem(id: "f3", title: "F3", keyName: "F3", width: 36, category: "fn"),
        KeyItem(id: "f4", title: "F4", keyName: "F4", width: 36, category: "fn"),
        KeyItem(id: "f5", title: "F5", keyName: "F5", width: 36, category: "fn"),
        KeyItem(id: "f6", title: "F6", keyName: "F6", width: 36, category: "fn"),
        KeyItem(id: "f7", title: "F7", keyName: "F7", width: 36, category: "fn"),
        KeyItem(id: "f8", title: "F8", keyName: "F8", width: 36, category: "fn"),
        KeyItem(id: "f9", title: "F9", keyName: "F9", width: 36, category: "fn"),
        KeyItem(id: "f10", title: "F10", keyName: "F10", width: 38, category: "fn"),
        KeyItem(id: "f11", title: "F11", keyName: "F11", width: 38, category: "fn"),
        KeyItem(id: "f12", title: "F12", keyName: "F12", width: 38, category: "fn"),

        // Power & Dismiss
        KeyItem(id: "cad", title: "Ctrl+Alt+Del", keyName: "CAD", width: 78, category: "danger"),
        KeyItem(id: "close", title: "✕", keyName: "Close", width: 32, category: "close")
    ]
    
    func setupTouchBar() {
        let tb = NSTouchBar()
        tb.delegate = self
        tb.defaultItemIdentifiers = [.init("rdp_sensei_scroll_bar")]
        self.touchBar = tb
    }
    
    func setupAppFocusObserver() {
        checkFrontmost()
        
        NSWorkspace.shared.notificationCenter.addObserver(
            forName: NSWorkspace.didActivateApplicationNotification,
            object: nil,
            queue: .main
        ) { [weak self] _ in
            self?.checkFrontmost()
        }
        
        NSWorkspace.shared.notificationCenter.addObserver(
            forName: NSWorkspace.didDeactivateApplicationNotification,
            object: nil,
            queue: .main
        ) { [weak self] _ in
            self?.checkFrontmost()
        }
    }
    
    func checkFrontmost() {
        guard let front = NSWorkspace.shared.frontmostApplication else { return }
        let name = (front.localizedName ?? "").lowercased()
        let bid = (front.bundleIdentifier ?? "").lowercased()
        
        // Frontmost must be Rdp Sensei or the browser hosting Rdp Sensei
        let isOurs = name.contains("rdp sensei") ||
                     name.contains("chrome") ||
                     name.contains("edge") ||
                     name.contains("brave") ||
                     name.contains("arc") ||
                     bid.contains("rdp-sensei")
                     
        isAppFrontmost = isOurs
        updateTouchBar()
    }
    
    func setSessionActive(_ active: Bool) {
        isSessionActive = active
        updateTouchBar()
    }
    
    func updateTouchBar() {
        if isSessionActive && isAppFrontmost {
            present()
        } else {
            dismiss()
        }
    }
    
    func touchBar(_ touchBar: NSTouchBar, makeItemForIdentifier identifier: NSTouchBarItem.Identifier) -> NSTouchBarItem? {
        if identifier.rawValue == "rdp_sensei_scroll_bar" {
            let item = NSCustomTouchBarItem(identifier: identifier)
            
            let stack = NSStackView()
            stack.orientation = .horizontal
            stack.spacing = 3.5
            stack.alignment = .centerY
            stack.distribution = .gravityAreas
            stack.translatesAutoresizingMaskIntoConstraints = false
            
            for k in keys {
                let btn = NSButton(title: k.title, target: self, action: #selector(buttonTapped(_:)))
                btn.identifier = NSUserInterfaceItemIdentifier(k.keyName)
                btn.bezelStyle = .rounded
                btn.font = NSFont.systemFont(ofSize: 12.5, weight: .semibold)
                
                switch k.category {
                case "danger":
                    btn.bezelColor = NSColor(red: 0.82, green: 0.20, blue: 0.20, alpha: 1.0)
                case "accent":
                    btn.bezelColor = NSColor(red: 0.12, green: 0.45, blue: 0.90, alpha: 1.0)
                case "nav":
                    btn.bezelColor = NSColor(red: 0.30, green: 0.32, blue: 0.36, alpha: 1.0)
                case "close":
                    btn.bezelColor = NSColor(white: 0.20, alpha: 1.0)
                default:
                    btn.bezelColor = NSColor(white: 0.24, alpha: 1.0)
                }
                
                btn.translatesAutoresizingMaskIntoConstraints = false
                btn.heightAnchor.constraint(equalToConstant: 30).isActive = true
                btn.widthAnchor.constraint(equalToConstant: k.width).isActive = true
                
                stack.addArrangedSubview(btn)
            }
            
            let scrollView = NSScrollView(frame: NSRect(x: 0, y: 0, width: 950, height: 30))
            scrollView.documentView = stack
            scrollView.hasHorizontalScroller = false
            scrollView.hasVerticalScroller = false
            scrollView.drawsBackground = false
            
            item.view = scrollView
            return item
        }
        return nil
    }
    
    func present() {
        guard let tb = touchBar, !isPresented else { return }
        let sel = NSSelectorFromString("presentSystemModalTouchBar:systemTrayItemIdentifier:")
        if NSTouchBar.responds(to: sel) {
            NSTouchBar.perform(sel, with: tb, with: "rdp.sensei.touchbar" as NSString)
            isPresented = true
            fputs("TOUCHBAR_STATUS:PRESENTED\n", stderr)
        }
    }
    
    func dismiss() {
        guard let tb = touchBar, isPresented else { return }
        let sel = NSSelectorFromString("dismissSystemModalTouchBar:")
        if NSTouchBar.responds(to: sel) {
            NSTouchBar.perform(sel, with: tb)
            isPresented = false
            fputs("TOUCHBAR_STATUS:DISMISSED\n", stderr)
        }
    }
    
    @objc func buttonTapped(_ sender: NSButton) {
        guard let keyName = sender.identifier?.rawValue else { return }
        if keyName == "Close" {
            dismiss()
            return
        }
        
        // Post key event to local Rdp Sensei Go server
        if let url = URL(string: "http://127.0.0.1:\(serverPort)/api/touchbar/press?key=\(keyName)") {
            var req = URLRequest(url: url)
            req.httpMethod = "POST"
            req.timeoutInterval = 1.0
            URLSession.shared.dataTask(with: req).resume()
        }
    }
    
    func startStdinLoop() {
        DispatchQueue.global(qos: .userInteractive).async {
            while let line = readLine() {
                let cmd = line.trimmingCharacters(in: .whitespacesAndNewlines)
                DispatchQueue.main.async {
                    switch cmd {
                    case "show":
                        self.setSessionActive(true)
                    case "hide":
                        self.setSessionActive(false)
                    case "quit":
                        self.dismiss()
                        exit(0)
                    default:
                        break
                    }
                }
            }
        }
    }
}

// Entry Point
let app = NSApplication.shared
let agent = TouchBarAgent.shared

// Parse port flag
let args = CommandLine.arguments
if let idx = args.firstIndex(of: "-port"), idx + 1 < args.count, let p = Int(args[idx + 1]) {
    agent.serverPort = p
}

agent.setupTouchBar()
agent.setupAppFocusObserver()
agent.startStdinLoop()

app.run()
