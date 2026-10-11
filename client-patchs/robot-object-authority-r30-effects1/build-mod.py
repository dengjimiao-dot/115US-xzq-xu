from pathlib import Path
import hashlib,json,zipfile
root=Path(__file__).resolve().parent;dist=root/'dist';dll=dist/'RobotObjectAuthority.dll'
m={'schema':2,'id':'qol.robot-object-authority','version':'0.1.0-r30-effects1-candidate','name':'Local robot object authority candidate',
'author':'115us local','description':'Owner-authorized local fake-party authority adapter; native gates preserved; manual acceptance pending.',
'requires':['qol.client-host'],'permissions':['client.file.write'],
'layers':{'client':{'ops':[{'kind':'file.add','target':'.115us-mods/RobotObjectAuthority.dll','source':'client/RobotObjectAuthority.dll','size':dll.stat().st_size,'sha256':hashlib.sha256(dll.read_bytes()).hexdigest()}]}}}
with zipfile.ZipFile(dist/'qol.robot-object-authority.mod.zip','w',zipfile.ZIP_DEFLATED) as z:
 z.writestr('mod.json',json.dumps(m,indent=2)+'\n');z.write(dll,'client/RobotObjectAuthority.dll');z.write(root/'README.md','README.md');z.write(root/'source-binding.json','evidence/source-binding.json')
print('mod built',dll.stat().st_size)
