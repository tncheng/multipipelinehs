from __future__ import print_function
import json
import sys

from matplotlib.pyplot import pink



def update_json(k,v):
    with open("update.json",'r') as cf:
        config=json.load(cf)
    config[k]=int(v)
    with open("update.json",'w') as cf:
        json.dump(config,cf,indent=4)
update_json(sys.argv[1],sys.argv[2])