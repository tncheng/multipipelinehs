import json
from operator import index
import os
from matplotlib.pyplot import flag, table
import pandas as pd

from numpy import mean,std,median
import json

result_dir='result'

data_tps={}
data_latency={}
max=0
table=[]
need_config=["rate","virtual_clients","byzNo"]
for f in os.listdir(result_dir):
    for re in os.listdir(f'{result_dir}/{f}'):
        if '.json' in re:
            with open(f'{result_dir}/{f}/{re}','r') as rj:
                result=json.load(rj)
                tpss=[]
                latencys=[]
                tx_rates=[]
                committx=[]
                forkedtx=[]
                abnormal=False
                
                for rn in result['result']:
                    if not rn:
                        continue
                    tpss.append(rn['tps(tx/s)'])
                    committx.append(rn['txs'])
                    forkedtx.append(rn['forkedtxs'])
                    try:
                        tx_rates.append(rn['tx_rate'])
                    except Exception as e:
                        print(f'error on {f}',e)
                        tx_rates.append(0)
                    if rn['latencys'] == '[]':
                        abnormal=True
                        continue
                    for la in rn['latencys'][1:-1].split(', '):
                        if la =='':
                            continue
                        latencys.append(float(la))
                    # latencys=latencys+[float(la) for la in rn['latencys'][1:-1].split(', ')]
                if len(latencys)>max:
                    max=len(latencys)
                data_tps[len(result['result'])]=mean(tpss)
                data_latency[len(result['result'])]=mean(latencys)
                mat="{:8}\t{:8}\t{:8}\t{:<16}\t{:<16}\t{:<16}\t{:<16}"

                try:
                    record=[result['result'][0]['replicas'],abnormal,mean(tx_rates)]+[result[co] for co in need_config]+[median(tpss)/1000,median(committx),median(forkedtx),median(latencys),mean(latencys),std(latencys)]
                    table.append(record)
                    # print(mat.format( result['result'][0]['replicas'],result['rate'],result['virtual_clients'],mean(tpss)/1000,median(latencys),mean(latencys),std(latencys)))
                except Exception as e:
                    print(f'error on {f}',e)
                
                
    # print(data_tps)
    # print(data_latency)
print(len(table))
tableframe=pd.DataFrame(table)
tableframe.to_excel(f'tps_latency.xls',header=None)
