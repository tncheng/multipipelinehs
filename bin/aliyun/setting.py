import pandas as pd

def load_instance_list(file):
    instance_table=pd.read_csv(file)
    size=instance_table.shape[0]
    instances=[]
    for i in range(size):
        inst={}
        inst['id']=instance_table.iloc[i,0]
        inst['pub_ip']=instance_table.iloc[i,9].split(';')[1]
        inst['int_ip']=instance_table.iloc[i,9].split(';')[0]
        instances.append(inst)
    return instances
