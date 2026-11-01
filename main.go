package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ---------- 配置 ----------
type Position struct {
	Name   string
	Code   string
	Cost   float64
	Shares int
}

var positions = []Position{
	{"北方稀土", "600111.SH", 17.8571, 300},
	{"紫金矿业", "601899.SH", 10.9901, 100},
	{"广州发展", "600098.SH", 4.5851, 1000},
	{"广汽集团", "601238.SH", 9.4302, 100},
	{"中国建筑", "601668.SH", 3.6193, 700},
	{"粤电力A", "000539.SZ", 4.6771, 700},
	{"东华能源", "002221.SZ", 9.7900, 200},
	{"南网能源", "003035.SZ", 5.7537, 700},
}

const dingtalkWebhook = "https://oapi.dingtalk.com/robot/send?access_token=9790819317f57b1fd03d3b011782c72619699dee04a2317b9f98722bff7109a2"

var code2pos = map[string]Position{}

type cred struct {
	BotToken     string `json:"bot_token"`
	ChatUserID   string `json:"chat_user_id"`
	BaseURL      string `json:"baseurl"`
	ContextToken string `json:"context_token"`
}

type bar struct {
	Date   string
	Close  float64
	Volume float64
}

func fatal(err error) {
	if err != nil {
		fmt.Println("错误:", err)
		os.Exit(1)
	}
}

func main() {
	exe, _ := os.Executable()
	base := filepath.Dir(exe)
	if _, err := os.Stat(filepath.Join(base, "bars.csv")); err != nil {
		base, _ = os.Getwd()
	}

	for _, p := range positions {
		code2pos[p.Code] = p
	}

	f, err := os.Open(filepath.Join(base, "bars.csv"))
	fatal(err)
	defer f.Close()
	rd := csv.NewReader(f)
	recs, err := rd.ReadAll()
	fatal(err)

	data := map[string][]bar{}
	var latest string
	for i, row := range recs {
		if i == 0 {
			continue // header
		}
		code, d := row[0], row[1]
		c, _ := strconv.ParseFloat(row[2], 64)
		v, _ := strconv.ParseFloat(row[3], 64)
		data[code] = append(data[code], bar{d, c, v})
		if d > latest {
			latest = d
		}
	}

	type result struct {
		Name  string
		Pnl   float64
		Chg   float64
		Price float64
		DD    float64
		State string
		Sig   []string
	}
	var results []result

	for code, bars := range data {
		p := code2pos[code]
		n := len(bars)
		closeT, closeY := bars[n-1].Close, bars[n-2].Close
		avg := func(vs []float64) float64 { s := 0.0; for _, v := range vs { s += v }; return s / float64(len(vs)) }
		closes := make([]float64, n)
		vols := make([]float64, n)
		for i, b := range bars {
			closes[i], vols[i] = b.Close, b.Volume
		}
		ma20 := avg(closes[n-20:])
		ma60 := avg(closes[n-60:])
		ma20y := avg(closes[n-21 : n-1])
		ma60y := avg(closes[n-61 : n-1])

		chg := (closeT/closeY - 1) * 100
		pnl := (closeT/p.Cost - 1) * 100
		high60 := math.Inf(-1)
		for _, c := range closes[n-60:] {
			if c > high60 {
				high60 = c
			}
		}
		dd := (closeT/high60 - 1) * 100
		low20 := closes[n-20]
		for _, c := range closes[n-20:] {
			if c < low20 {
				low20 = c
			}
		}
		avgVol := 0.0
		for i := n - 21; i < n-1; i++ {
			avgVol += vols[i]
		}
		avgVol /= 20
		vr := 0.0
		if avgVol > 0 {
			vr = vols[n-1] / avgVol
		}

		var sig []string
		if pnl > 20 && closeT < ma20 && ma20 < ma60 {
			sig = append(sig, "厚利破位(价<MA20<MA60)，减仓止盈")
		}
		if pnl > 15 && dd <= -10 {
			sig = append(sig, fmt.Sprintf("高点回撤%.0f%%，移动止盈", dd))
		}
		if pnl < -10 && closeT <= low20*1.005 && closeT < ma60 {
			sig = append(sig, "套牢创20日新低，止损/换股")
		}
		if closeT < ma60 && closeY >= ma60y {
			sig = append(sig, "今日跌破MA60")
		}
		if ma20 < ma60 && ma20y >= ma60y {
			sig = append(sig, "MA20死叉MA60")
		}
		if chg <= -5 && vr >= 1.5 {
			sig = append(sig, fmt.Sprintf("放量急跌%.1f%%", chg))
		}

		state := "震荡"
		if closeT > ma20 && ma20 > ma60 {
			state = "持有(多头)"
		} else if closeT < ma20 && ma20 < ma60 {
			state = "空头"
		}

		results = append(results, result{p.Name, pnl, chg, closeT, dd, state, sig})
	}

	sort.Slice(results, func(i, j int) bool { return results[i].Pnl > results[j].Pnl })

	fmt.Printf("盯盘日期: %s  持仓 %d 只\n", latest, len(positions))
	for _, r := range results {
		s := "无"
		if len(r.Sig) > 0 {
			s = strings.Join(r.Sig, "；")
		}
		fmt.Printf("  %s\t现价%.2f\t日%.2f%%\t盈亏%.1f%%\t%s\t%s\n",
			r.Name, r.Price, r.Chg, r.Pnl, r.State, s)
	}

	var alarm []result
	for _, r := range results {
		if len(r.Sig) > 0 {
			alarm = append(alarm, r)
		}
	}
	if len(alarm) == 0 {
		fmt.Printf("今日无卖出信号。最强：%s，最弱：%s\n", results[0].Name, results[len(results)-1].Name)
		return
	}

	lines := []string{fmt.Sprintf("【盯盘】%s 收盘，%d 只触发信号：", latest, len(alarm))}
	for _, r := range alarm {
		fmt.Printf("  -> %s(%.0f%%): %s\n", r.Name, r.Pnl, strings.Join(r.Sig, "；"))
		lines = append(lines, fmt.Sprintf("· %s(%.2f元, 盈亏%+.0f%%)：%s", r.Name, r.Price, r.Pnl, strings.Join(r.Sig, "；")))
	}
	lines = append(lines, "仅供参考，不构成投资建议。")
	msg := strings.Join(lines, "\n")

	fmt.Printf("[钉钉] %v\n", pushDingtalk(msg))
	fmt.Printf("[微信] %v\n", pushWechat(filepath.Join(base, "ilink_cred.json"), msg))
}

func pushDingtalk(text string) bool {
	b, _ := json.Marshal(map[string]any{"msgtype": "text", "text": map[string]string{"content": text}})
	resp, err := http.Post(dingtalkWebhook, "application/json", bytes.NewReader(b))
	if err != nil {
		fmt.Println("[钉钉] 失败:", err)
		return false
	}
	defer resp.Body.Close()
	rb, _ := io.ReadAll(resp.Body)
	return bytes.Contains(rb, []byte(`"errcode":0`))
}

func pushWechat(credPath, text string) bool {
	cb, err := os.ReadFile(credPath)
	if err != nil {
		fmt.Println("[微信] 无凭据:", err)
		return false
	}
	var c cred
	if err := json.Unmarshal(cb, &c); err != nil {
		fmt.Println("[微信] 凭据解析失败:", err)
		return false
	}
	clientID := fmt.Sprintf("%d-%d", rand.Intn(1<<31), time.Now().UnixNano())
	body := map[string]any{
		"base_info": map[string]string{"channel_version": "2.0.0"},
		"msg": map[string]any{
			"from_user_id":  "",
			"to_user_id":    c.ChatUserID,
			"client_id":     clientID,
			"message_type":  2,
			"message_state": 2,
			"context_token": c.ContextToken,
			"item_list": []map[string]any{
				{"type": 1, "text_item": map[string]string{"text": text}},
			},
		},
	}
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", c.BaseURL+"/ilink/bot/sendmessage", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.BotToken)
	req.Header.Set("AuthorizationType", "ilink_bot_token")
	req.Header.Set("X-WECHAT-UIN", fmt.Sprintf("%d", rand.Intn(1<<31)))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println("[微信] 失败:", err)
		return false
	}
	defer resp.Body.Close()
	rb, _ := io.ReadAll(resp.Body)
	return bytes.Contains(rb, []byte("message_id"))
}
