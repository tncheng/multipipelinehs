import json
from operator import index
import os
from matplotlib.pyplot import flag, table
import pandas as pd

from numpy import append
from numpy import mean
from numpy import var
import json

result_dir='result-mthotstuff'

data_tps={}
data_latency={}
max=0
for f in os.listdir(result_dir):
    if '.xls' in f:
        continue
    for re in os.listdir(f'{result_dir}/{f}'):
        if '.json' in re:
            with open(f'{result_dir}/{f}/{re}','r') as rj:
                result=json.load(rj)
                tpss=[]
                latencys=[]
                
                for rn in result['result']:
                    if not rn:
                        continue
                    tpss.append(rn['tps(tx/s)'])
                    for la in rn['latencys'][1:-1].split(', '):

                        if la =='':
                            continue
                        latencys.append(float(la))
                    # latencys=latencys+[float(la) for la in rn['latencys'][1:-1].split(', ')]
                if len(latencys)>max:
                    max=len(latencys)
                data_tps[len(result['result'])]=mean(tpss)
                data_latency[len(result['result'])]=latencys
                print(result['result'][0]['replicas'],mean(tpss)/1000,mean(latencys))
1/0
print(max)
for k,v in data_latency.items():
    data_latency[k]=data_latency[k]+[0 for _ in range(max-len(data_latency[k]))]
    print(len(data_latency[k]))

table_tps=pd.DataFrame(data_tps,index=[0])
table_latency=pd.DataFrame(data_latency)
table_tps.to_excel(f'{result_dir}/tps.xls')
table_latency.to_excel(f'{result_dir}/latency.xls')
