from pathlib import Path
import hashlib,json,zipfile

HERE=Path(__file__).resolve().parent
dll=HERE/'dist/HeadTextUpscale.dll'
manifest={'schema':2,'id':'qol.head-text-upscale-exclusion','version':'0.3.0-candidate-r3',
          'name':'头顶与菜单文字避开画质提升（候选 r3）','author':'本地调整',
          'description':'在头顶和主菜单绘制范围内，完整创建字体单元后交给原生增强之后的叠加层；保留字体、坐标、画面设置，待实机验证。',
          'requires':['qol.client-host'],'permissions':['client.file.write'],
          'layers':{'client':{'ops':[{'kind':'file.add','target':'.115us-mods/HeadTextUpscale.dll',
          'source':'client/HeadTextUpscale.dll','size':dll.stat().st_size,
          'sha256':hashlib.sha256(dll.read_bytes()).hexdigest()}]}}}
package=HERE/'dist/head-text-upscale-r3.mod.zip'
with zipfile.ZipFile(package,'w',zipfile.ZIP_DEFLATED) as archive:
    archive.writestr('mod.json',json.dumps(manifest,ensure_ascii=False,indent=2)+'\n')
    archive.write(dll,'client/HeadTextUpscale.dll');archive.write(HERE/'README.md','README.md')
(HERE/'dist/package.json').write_text(json.dumps({'package':str(package),
    'package_sha256':hashlib.sha256(package.read_bytes()).hexdigest(),
    'dll_sha256':manifest['layers']['client']['ops'][0]['sha256']},indent=2),encoding='utf8')
print(package)
