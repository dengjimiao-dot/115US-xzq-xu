from pathlib import Path
import sys,hashlib,json
root=Path(__file__).resolve().parent

import pefile,capstone
exe=Path(sys.argv[1]) if len(sys.argv)>1 else sys.exit('Usage: python generate-evidence.py PATH_TO_DFO_EXE');sha=hashlib.sha256(exe.read_bytes()).hexdigest()
assert sha=='515fefa1c71577a15f90513f860270320f3502bc492ad596f10cf176b91c21eb'
p=pefile.PE(str(exe),fast_load=True);md=capstone.Cs(capstone.CS_ARCH_X86,capstone.CS_MODE_64)
spec=[('Authority',0x5b89780,16),('State',0x59a90f0,16),('LocalSeat',0x59a9d30,16),('Actor',0x59a9de0,16),('Identity',0x5b8b0a0,16),('EffectOwner',0x5ba4c20,16)]
out=['#pragma once',f'static const DWORD kImageTimestamp={p.FILE_HEADER.TimeDateStamp}u;',f'static const DWORD kImageSize={p.OPTIONAL_HEADER.SizeOfImage}u;']
binding={'client_sha256':sha,'functions':{},'scope':'local lease plus native same-map type3/4 and verified native fake-parent type17 entities; regular player entity unchanged'}
for name,rva,size in spec:
 b=p.get_data(rva,size)
 if name=='Authority':
  ins=list(md.disasm(b,p.OPTIONAL_HEADER.ImageBase+rva))
  assert sum(i.size for i in ins)==size
  assert [(i.mnemonic,i.op_str) for i in ins]==[('mov','rax, rsp'),('push','rdi'),('push','r12'),('push','r13'),('push','r14'),('push','r15'),('sub','rsp, 0x40')]
 out.extend([f'static const uintptr_t k{name}RVA={hex(rva)};',f'static const unsigned char k{name}Head[]={{'+','.join(hex(x) for x in b)+'};'])
 binding['functions'][name]={'rva':hex(rva),'bytes':b.hex()}
(root/'dist').mkdir(exist_ok=True)
(root/'dist/native_evidence.h').write_text('\n'.join(out)+'\n',encoding='utf-8')
(root/'dist/source-binding.json').write_text(json.dumps(binding,indent=2)+'\n',encoding='utf-8')
print('pinned native bytes verified')
