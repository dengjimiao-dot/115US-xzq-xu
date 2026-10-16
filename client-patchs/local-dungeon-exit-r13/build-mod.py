from pathlib import Path
import hashlib,json,zipfile
root=Path(__file__).resolve().parent;dist=root/'dist';dll=dist/'LocalDungeonExit.dll'
m={'schema':2,'id':'qol.local-dungeon-exit','version':'0.1.0-r13-candidate','name':'Local dungeon exit permission candidate',
'author':'115us local','description':'Owner-authorized local return-town/end-game permissions; manual acceptance pending.',
'requires':['qol.client-host'],'permissions':['client.file.write'],
'layers':{'client':{'ops':[{'kind':'file.add','target':'.115us-mods/LocalDungeonExit.dll','source':'client/LocalDungeonExit.dll','size':dll.stat().st_size,'sha256':hashlib.sha256(dll.read_bytes()).hexdigest()}]}}}
with zipfile.ZipFile(dist/'qol.local-dungeon-exit.mod.zip','w',zipfile.ZIP_DEFLATED) as z:
 z.writestr('mod.json',json.dumps(m,indent=2)+'\n');z.write(dll,'client/LocalDungeonExit.dll');z.write(root/'README.md','README.md');z.write(root/'source-binding.json','evidence/source-binding.json')
print(json.dumps(m,indent=2))
