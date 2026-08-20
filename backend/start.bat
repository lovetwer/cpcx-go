@echo off
chcp 65001 >nul
cd /d D:\Project\cpcxnew\backend
echo ====================================
echo   大奖来了后端启动中...
echo   网页访问: http://localhost:8080
echo   关闭此窗口即停止服务
echo ====================================
lottery-manager.exe
pause
