package main

import (
	"io"
	"net/http"
)

// handleRecognize 上传彩票截图，调用 OCR 识别并自动录入。
// 同时挂载在 /api/lottery/recognize 与 /lottery/ai-generate 两个路径。
func handleRecognize(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "请上传图片文件")
		return
	}
	file, _, err := r.FormFile("image")
	if err != nil {
		writeError(w, http.StatusBadRequest, "缺少 image 字段")
		return
	}
	defer file.Close()
	buf, err := io.ReadAll(file)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取图片失败")
		return
	}
	results, err := RecognizeLottery(buf, "upload.jpg")
	if err != nil {
		writeError(w, http.StatusBadGateway, "识别失败："+err.Error())
		return
	}

	uid := currentUserID(r)
	// dry_run=1：只解析返回号码，不入库，便于前端在「录入」页预填选球后让用户确认保存
	dryRun := r.FormValue("dry_run") == "1" || r.FormValue("dry_run") == "true"
	var inserted, skipped, parsed []map[string]interface{}
	for _, res := range results {
		red, blue, ok := validateBalls(res.Type, res.RedBalls, res.BlueBalls)
		if !ok {
			skipped = append(skipped, map[string]interface{}{
				"issue": res.Issue, "type": res.Type,
				"reason": "号码数量不符（双色球需6红+1蓝，大乐透需5前+2后）",
			})
			continue
		}
		// 期号未识别：回退到最近一期已开奖的开奖日期，便于预览/入库后参与核对
		if res.Issue == "" {
			if latest, ok2 := latestDrawDate(res.Type); ok2 {
				res.Issue = latest
			}
		}
		if dryRun {
			parsed = append(parsed, map[string]interface{}{
				"type": res.Type, "issue": res.Issue,
				"red_balls": red, "blue_balls": blue,
			})
			continue
		}
		lot := Lottery{UserID: uid, Type: res.Type, Issue: res.Issue, RedBalls: red, BlueBalls: blue, Status: StatusPending}
		if err := insertLottery(&lot); err != nil {
			skipped = append(skipped, map[string]interface{}{"issue": res.Issue, "reason": "写入失败"})
			continue
		}
		// 识别录入后立即尝试核对（若该期已开奖）
		checkOne(&lot)
		inserted = append(inserted, map[string]interface{}{
			"id": lot.ID, "type": lot.Type, "issue": lot.Issue,
			"red_balls": lot.RedBalls, "blue_balls": lot.BlueBalls, "status": lot.Status,
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":       true,
		"dry_run":  dryRun,
		"inserted": inserted,
		"parsed":   parsed,
		"skipped":  skipped,
		"message":  map[bool]string{true: "识别完成，请在录入页确认", false: "识别完成，已自动录入"}[dryRun],
	})
}
