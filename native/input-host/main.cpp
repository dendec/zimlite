// Headless m17n input worker. Protocol fields are hex-encoded UTF-8, bounded.
#include <m17n.h>
#include <anthy/anthy.h>
#include <unicode/ubrk.h>
#include <unicode/ustring.h>
#include <algorithm>
#include <cstdlib>
#include <iostream>
#include <sstream>
#include <string>
#include <vector>
#include <stdexcept>

static MText* text;
static bool preview=false;
static std::string utf8(MText* mt) {
 unsigned char buf[16384];int n=mconv_encode_buffer(Mcoding_utf_8,mt,buf,sizeof(buf));
 if(n<0)throw std::runtime_error("UTF-8 conversion failed");
 return std::string(reinterpret_cast<char*>(buf),n);
}
static MText* from_utf8(const std::string& s) {
 auto* mt=mconv_decode_buffer(Mcoding_utf_8,reinterpret_cast<const unsigned char*>(s.data()),s.size());
 if(!mt)throw std::runtime_error("invalid UTF-8");
 return mt;
}
static std::string hex(const std::string& s){
 const char* h="0123456789abcdef";std::string out;out.reserve(s.size()*2);
 for(unsigned char c:s){out+=h[c>>4];out+=h[c&15];}return out;
}
static std::string unhex(const std::string& s){
 if(s.size()>8192||s.size()%2)throw std::runtime_error("invalid request size");
 auto digit=[](char c){if(c>='0'&&c<='9')return c-'0';if(c>='a'&&c<='f')return c-'a'+10;throw std::runtime_error("invalid hex");};
 std::string out;for(size_t i=0;i<s.size();i+=2)out+=static_cast<char>(digit(s[i])*16+digit(s[i+1]));return out;
}
static void surrounding(MInputContext* ic,MSymbol command){
 int count=static_cast<int>(reinterpret_cast<intptr_t>(mplist_value(ic->plist)));
 int length=preview?0:mtext_len(text);
 if(command==Minput_get_surrounding_text){
  MText* mt=mtext();
  if(count<0&&!preview){auto* part=mtext_duplicate(text,std::max(0,length+count),length);mtext_cat(mt,part);m17n_object_unref(part);}
  mplist_set(ic->plist,Mtext,mt);m17n_object_unref(mt);
 } else if(command==Minput_delete_surrounding_text&&!preview&&count<0){mtext_del(text,std::max(0,length+count),length);}
}
static void append(MText* value){
 if(utf8(text).size()+utf8(value).size()<=2048)mtext_cat(text,value);
}
static void key(MInputContext* ic,const std::string& k){
 MSymbol sym=k.empty()?Mnil:msymbol(k.c_str());
 if(!minput_filter(ic,sym,nullptr)){
  MText* out=mtext();int ret=minput_lookup(ic,sym,nullptr,out);append(out);m17n_object_unref(out);
  if(ret!=0&&k.size()==1&&static_cast<unsigned char>(k[0])>=32&&utf8(text).size()<2048)mtext_cat_char(text,k[0]);
 }
}
static void erase(){
 std::string s=utf8(text);if(s.empty())return;
 UChar wide[2049];int32_t len=0;UErrorCode status=U_ZERO_ERROR;
 u_strFromUTF8(wide,2049,&len,s.data(),s.size(),&status);
 UBreakIterator* it=ubrk_open(UBRK_CHARACTER,nullptr,wide,len,&status);
 if(U_FAILURE(status))throw std::runtime_error("Unicode segmentation failed");
 ubrk_last(it);int boundary=ubrk_previous(it);ubrk_close(it);
 int32_t bytes=0;char result[8192];status=U_ZERO_ERROR;
 u_strToUTF8(result,sizeof(result),&bytes,wide,std::max(0,boundary),&status);
 if(U_FAILURE(status))throw std::runtime_error("Unicode segmentation failed");
 m17n_object_unref(text);text=from_utf8(std::string(result,bytes));
}
static void state(MInputContext* ic,const std::vector<std::string>& labels={}){
 std::vector<std::string> candidates;int selected=0;int base=0;
 for(MPlist* p=ic->candidate_list;p&&mplist_key(p)!=Mnil;p=mplist_next(p)){
  std::vector<std::string> group;
  if(mplist_key(p)==Mtext){auto* mt=static_cast<MText*>(mplist_value(p));for(int i=0;i<mtext_len(mt);++i){auto* one=mtext();mtext_cat_char(one,mtext_ref_char(mt,i));group.push_back(utf8(one));m17n_object_unref(one);}}
  else if(mplist_key(p)==Mplist){for(MPlist* q=static_cast<MPlist*>(mplist_value(p));mplist_key(q)!=Mnil;q=mplist_next(q))if(mplist_key(q)==Mtext)group.push_back(utf8(static_cast<MText*>(mplist_value(q))));}
  if(ic->candidate_index<base+static_cast<int>(group.size())){
   int local=std::max(0,ic->candidate_index-base);int start=local/8*8;selected=local-start;
   for(int i=start;i<std::min(start+8,static_cast<int>(group.size()));++i)candidates.push_back(group[i]);
   break;
  }base+=group.size();
 }
 if(!ic->candidate_show)candidates.clear();
 std::cout<<"S\t"<<hex(utf8(text))<<"\t"<<hex(utf8(ic->preedit))<<"\t"<<selected<<"\t"<<candidates.size();
 for(const auto& s:candidates)std::cout<<"\t"<<hex(s);
 for(const auto& s:labels)std::cout<<"\t"<<hex(s);
 std::cout<<'\n'<<std::flush;
}
int main(int argc,char** argv){
 try{
  if(argc!=5)throw std::runtime_error("expected language, method, text and runtime directory");
  std::string root=argv[4];
  setenv("M17NDIR",(root+"/db").c_str(),1);
  setenv("M17N_INPUT_MODULE_DIR",(root+"/lib").c_str(),1);
  anthy_conf_override("ANTHYDIR",(root+"/anthy").c_str());
  anthy_conf_override("DIC_FILE",(root+"/anthy/anthy.dic").c_str());
  anthy_conf_override("ZIPDICT",(root+"/anthy/zipcode.t").c_str());
  M17N_INIT();text=from_utf8(unhex(argv[3]));
  mplist_put_func(minput_default_driver.callback_list,Minput_get_surrounding_text,reinterpret_cast<M17NFunc>(surrounding));
  mplist_put_func(minput_default_driver.callback_list,Minput_delete_surrounding_text,reinterpret_cast<M17NFunc>(surrounding));
  MInputMethod* im=minput_open_im(msymbol(argv[1]),msymbol(argv[2]),nullptr);
  if(!im)throw std::runtime_error("input method unavailable");
  std::vector<std::string> labels;
  std::string method=argv[2];
  if(method=="kbd"||method=="han2"||method=="azerty"||method.find("isiri")==0||method.find("inscript")==0||method=="phonetic"){
   std::string keys="qwertyuiop[]asdfghjkl;'\\zxcvbnm,./-=";
   for(int layer=0;layer<2;++layer)for(char c:keys){
    if(layer){
     if(c>='a'&&c<='z')c-=32;
     else {const std::string plain="[];'\\,./-=",upper="{}:\"|<>?_+";auto pos=plain.find(c);if(pos!=std::string::npos)c=upper[pos];}
    }
    preview=true;MInputContext* tmp=minput_create_ic(im,nullptr);MText* value=mtext();
    MSymbol sym=msymbol(std::string(1,c).c_str());if(!minput_filter(tmp,sym,nullptr))minput_lookup(tmp,sym,nullptr,value);
    std::string label=utf8(mtext_len(value)?value:tmp->preedit);
    labels.push_back(label.empty()||label.size()>24?std::string(1,c):label);
    m17n_object_unref(value);minput_destroy_ic(tmp);preview=false;
   }
  }
  MInputContext* ic=minput_create_ic(im,nullptr);if(!ic)throw std::runtime_error("cannot create input context");
  state(ic,labels);
  std::string line;
  while(std::getline(std::cin,line)){
   if(line.size()>8300)throw std::runtime_error("request too long");
   size_t tab=line.find('\t');std::string op=line.substr(0,tab);std::string value=tab==std::string::npos?"":unhex(line.substr(tab+1));
   if(op=="key")key(ic,value);
   else if(op=="replace"){if(value.size()>2048)throw std::runtime_error("input too long");m17n_object_unref(text);text=from_utf8(value);}
   else if(op=="commit")key(ic,"");
   else if(op=="cancel")minput_reset_ic(ic);
   else if(op=="backspace"){if(mtext_len(ic->preedit))key(ic,"BackSpace");else erase();}
   else if(op=="text"){key(ic,"");auto* v=from_utf8(value);append(v);m17n_object_unref(v);}
   else throw std::runtime_error("unknown request");
   state(ic);
  }
  minput_destroy_ic(ic);minput_close_im(im);m17n_object_unref(text);M17N_FINI();return 0;
 }catch(const std::exception& e){std::cout<<"E\t"<<hex(e.what())<<'\n'<<std::flush;return 1;}
}
