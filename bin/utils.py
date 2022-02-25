
import time

import json

from numpy import nan_to_num

def UpdateConfig(dir):
    with open("update.json",'r') as uc:
        config=json.load(uc)
    with open("config.json",'r') as oc:
        origin_config=json.load(oc)
    
    for k,v in config.items():
        origin_config[k]=v
    with open("{}/config.json".format(dir),'w') as wf:
        json.dump(origin_config,wf,indent=4)
        


def Analyze(config_dir,result_dir,log):
    txs=0
    fp=open("{}/{}".format(result_dir, log))
    timelist=[]
    latencys=0.0
    blocks=0
    stat={}
    alg=""
    for line in iter(fp):
        if "consensus" in line:
            alg=line.split(':')[-1]
        if "committed" in line:
            a=line.split(' ')[1]+'-'+line.split(' ')[2].split('.')[0]
            latencys+=nan_to_num(float(line.split(' ')[20]))
            st=time.mktime(time.strptime(a,"%Y/%m/%d-%H:%M:%S"))
            timelist.append(st)
            commitedtx=line.split(':')[5][:-6]
            txs+=int(commitedtx)
            if int(commitedtx)==0:
                continue
            blocks+=1
    if len(timelist)<1:
        print("there is no valid log")
        return None
    duration=timelist[-1]-timelist[0]
    stat['algorithm']=alg[:-1]
    stat['running time(s)']=duration
    stat['txs']=txs
    stat['tps(tx/s)']=int(txs/duration)
    stat['latency(ms)']=latencys/blocks
    print(json.dumps( stat,indent=2))

    currentConfig=None
    with open("{}/config.json".format(config_dir),'r') as cf:
        currentConfig=json.load(cf)
        stat['input rate(tx/s)']=int(currentConfig['duplicate']/(currentConfig['txinterval']*1e-6))
    currentConfig["result"]=stat
    result_file=".".join(log.split('.')[:-1])+".result.json"
    with open("{}/{}".format(result_dir,result_file),'w') as wf:
        json.dump(currentConfig,wf,indent=4)