# -*- coding: utf-8 -*-
"""把8只自选股最近120个交易日导出为 bars.csv，供 Go 版盯盘程序读取。"""
import duckdb, os
HERE = os.path.dirname(os.path.abspath(__file__))
DB = r"D:\code\fintech-ths\data\market.duckdb"
codes = ["600111.SH","601899.SH","600098.SH","601238.SH",
         "601668.SH","000539.SZ","002221.SZ","003035.SZ"]
cl = ",".join(f"'{c}'" for c in codes)
con = duckdb.connect(DB, read_only=True)
df = con.execute(f"""
  WITH t AS (SELECT thscode, date, close, volume,
                    ROW_NUMBER() OVER (PARTITION BY thscode ORDER BY date DESC) rn
             FROM v_daily_qfq WHERE thscode IN ({cl}))
  SELECT thscode, strftime(date,'%Y-%m-%d') d, close, volume FROM t WHERE rn<=120
  ORDER BY thscode, date
""").fetchdf()
con.close()
df.to_csv(os.path.join(HERE, "bars.csv"), index=False)
print(f"导出 {len(df)} 行 -> bars.csv, 最新 {df['d'].max()}")
