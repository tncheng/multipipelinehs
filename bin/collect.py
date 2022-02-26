import json
from operator import index
import os
import pandas as pd

from numpy import append
dir='result'
mem=[]
tps=[]
alg=''
b_size=0
for i in os.listdir(dir):
    if 'json' in i:
        with open(f'{dir}/{i}','r') as rj:
            result=json.load(rj)
            mem.append(result['memsize'])
            tps.append(result['result']['tps(tx/s)'])
            alg=result['result']['algorithm']
            b_size=result['bsize']
            

dataframe=pd.DataFrame({'memsize':mem,'tps':tps})
dataframe.to_csv(f'{alg}_tps_b_{b_size}_mem.csv',index=False,sep=',')
            