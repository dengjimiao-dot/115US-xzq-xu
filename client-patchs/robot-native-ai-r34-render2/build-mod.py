from pathlib import Path
import hashlib,json,zipfile
root=Path(__file__).resolve().parent;dist=root/'dist';dll=dist/'RobotNativeAI.dll'
m={'schema':2,'id':'qol.robot-native-ai','version':'0.1.0-r34-render2','name':'Local ordinary robot native AI candidate',
   'author':'115us local','description':'Explicit local ordinary fake-player native AI adapter; manual acceptance pending.',
   'requires':['qol.client-host'],'permissions':['client.file.write'],
   'layers':{'client':{'ops':[{'kind':'file.add','target':'.115us-mods/RobotNativeAI.dll','source':'client/RobotNativeAI.dll','size':dll.stat().st_size,'sha256':hashlib.sha256(dll.read_bytes()).hexdigest()}]}}}
with zipfile.ZipFile(dist/'qol.robot-native-ai.mod.zip','w',zipfile.ZIP_DEFLATED) as z:
    z.writestr('mod.json',json.dumps(m,indent=2)+'\n');z.write(dll,'client/RobotNativeAI.dll')
    z.write(root/'README.md','README.md');z.write(root/'source-binding.json','evidence/source-binding.json')
print('mod built',dll.stat().st_size)
