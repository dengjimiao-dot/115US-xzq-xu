from pathlib import Path
import hashlib,json,zipfile
R=Path(__file__).parent
dll=R/'dist/dinput8.dll'
digest=hashlib.sha256(dll.read_bytes()).hexdigest()
manifest={'schema':2,'id':'qol.avatar-equipped-marker','version':'1.1.1-head-slot-fix-candidate',
 'name':'装扮来源绿色人形角标（候选）','author':'本地整合',
 'description':'原生第403帧装扮来源角标；修复首次进城头顶私有字体测量误用第37号空槽，统一测量/绘制至已注册第40号槽。待用户实机验收。',
 'permissions':['client.file.write'],'layers':{'client':{'ops':[{
 'kind':'file.replace','target':'dinput8.dll','source':'client/dinput8.dll',
 'size':dll.stat().st_size,'sha256':digest,'note':'Existing combined DLL; native avatar frame 403 retained. Private head font metrics and glyph keys use registered slot 40. Backup and exact uninstall supported.'}]}}}
out=R/'dist/avatar-equipped-marker.mod.zip'
with zipfile.ZipFile(out,'w',zipfile.ZIP_DEFLATED) as z:
 z.writestr('mod.json',json.dumps(manifest,ensure_ascii=False,indent=2)+'\n')
 z.write(dll,'client/dinput8.dll');z.write(R/'README.md','README.md')
print(json.dumps({'package':str(out),'dll_sha256':digest,'bytes':dll.stat().st_size},ensure_ascii=False))
