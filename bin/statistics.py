import os
import time

import json

stat={}

logs=[]
for root,dirs,files in os.walk('./'):
    for f in files:
        if f.split('.')[-1]=='log' and f.split('.')[0] == 'server':
            logs.append(f)

for f in logs:
    print(f)
    fp=open(f)
    txs=0
    timelist=[]
    for line in iter(fp):
        if "committed" in line:
            a=line.split(' ')[1]+'-'+line.split(' ')[2].split('.')[0]
            # print(a)
            st=time.mktime(time.strptime(a,"%Y/%m/%d-%H:%M:%S"))
            timelist.append(st)
            commitedtx=line.split(':')[5][:-6]
            txs+=int(commitedtx)
    if len(timelist)<1:
        print("there is no valid log")
        break
    duration=timelist[-1]-timelist[0]
    stat['running time(s)']=duration
    stat['txs']=txs
    stat['tps(tx/s)']=int(txs/duration)
    print(json.dumps( stat,indent=2))

    currentConfig=None
    with open("config.json",'r') as cf:
        currentConfig=json.load(cf)
        stat['input rate(tx/s)']=int(currentConfig['duplicate']/(currentConfig['txinterval']*1e-6))
    currentConfig["result"]=stat
    with open("result_{}.json".format(stat['tps(tx/s)']),'w') as wf:
        json.dump(currentConfig,wf,indent=4)
        

    break