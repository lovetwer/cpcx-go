@echo off
REM 前后端分离开发一键启动
REM   后端(Go)  -> http://localhost:8080  (只提供 API)
REM   前端(Vite)-> http://localhost:5173  (开发热更新, 通过 proxy 调 8080)
REM 会弹出两个独立窗口, 关闭窗口即停止对应服务。

title 彩票管家 - 后端(8080)
cd /d D:\Project\cpcxnew\backend
start "backend-8080" lottery-manager.exe

cd /d D:\Project\cpcxnew\web
start "frontend-5173" cmd /k npm run dev

echo 已启动前后端, 浏览器打开 http://localhost:5173
pause
