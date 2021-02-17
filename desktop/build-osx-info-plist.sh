#!/bin/bash
cat << EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleExecutable</key>
    <string>bbgo-desktop</string>
    <key>CFBundleIconFile</key>
    <string>icon.icns</string>
    <key>CFBundleIdentifier</key>
    <string>com.bbgo.lorca</string>
    <key>LSFileQuarantineEnabled</key>
    <true/>
    <key>NSUserNotificationAlertStyle</key>
    <string>banner</string>
    <key>NSHighResolutionCapable</key>
    <true/>
</dict>
</plist>
EOF
