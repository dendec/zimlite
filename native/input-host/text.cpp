// Small relocatable shaping backend for the SDL keyboard only.
#include <ft2build.h>
#include FT_FREETYPE_H
#include <hb.h>
#include <hb-ft.h>
#include <fribidi.h>
#include <algorithm>
#include <cstdint>
#include <cstdlib>
#include <map>
#include <string>
#include <vector>

static FT_Library library=nullptr;
static std::map<std::string,FT_Face> faces;
extern "C" void kb_clear(){for(auto& kv:faces)FT_Done_Face(kv.second);faces.clear();if(library)FT_Done_FreeType(library);library=nullptr;}
static FT_Face face(const std::string& file,int size){
 if(!library&&FT_Init_FreeType(&library))return nullptr;
 auto it=faces.find(file);
 if(it==faces.end()){FT_Face f;if(FT_New_Face(library,file.c_str(),0,&f))return nullptr;it=faces.emplace(file,f).first;}
 FT_Set_Pixel_Sizes(it->second,0,size);return it->second;
}
struct Run{int start,end;FriBidiLevel level;hb_script_t script;};
struct Glyph{FT_Face face;unsigned id;int x,y;};
extern "C" unsigned char* kb_render(const char* root,const char* input,int size,int maxWidth,int* width,int* height){
 if(!input||!root||size<8||size>32||maxWidth<1||maxWidth>4096)return nullptr;
 std::vector<FriBidiChar> chars; // Decode through HarfBuzz's UTF-8 decoder.
 hb_buffer_t* decode=hb_buffer_create();hb_buffer_add_utf8(decode,input,-1,0,-1);
 unsigned count=0;auto* info=hb_buffer_get_glyph_infos(decode,&count);if(count>2048){hb_buffer_destroy(decode);return nullptr;}
 for(unsigned i=0;i<count;++i)chars.push_back(info[i].codepoint);
 hb_buffer_destroy(decode);
 if(chars.empty())return nullptr;
 std::vector<FriBidiCharType> types(chars.size());std::vector<FriBidiLevel> levels(chars.size());
 fribidi_get_bidi_types(chars.data(),chars.size(),types.data());FriBidiParType direction=FRIBIDI_PAR_ON;
 if(!fribidi_get_par_embedding_levels(types.data(),chars.size(),&direction,levels.data()))return nullptr;
 auto* unicode=hb_unicode_funcs_get_default();std::vector<Run> runs;
 hb_script_t previous=HB_SCRIPT_LATIN;
 for(size_t i=0;i<chars.size();++i){auto script=hb_unicode_script(unicode,chars[i]);
  if(script==HB_SCRIPT_COMMON||script==HB_SCRIPT_INHERITED)script=previous;
  if(runs.empty()||runs.back().script!=script||runs.back().level!=levels[i])runs.push_back({static_cast<int>(i),static_cast<int>(i+1),levels[i],script});else runs.back().end=i+1;
  previous=script;
 }
 int highest=0,lowestOdd=126;for(auto r:runs){highest=std::max(highest,int(r.level));if(r.level%2)lowestOdd=std::min(lowestOdd,int(r.level));}
 for(int level=highest;level>=lowestOdd;--level)for(size_t i=0;i<runs.size();){if(runs[i].level<level){++i;continue;}size_t end=i;while(end<runs.size()&&runs[end].level>=level)++end;std::reverse(runs.begin()+i,runs.begin()+end);i=end;}
 std::vector<Glyph> glyphs;int pen=0,minY=0,maxY=size,minX=0,maxX=0;
 for(auto run:runs){
  FT_Face f=face(std::string(root)+"/unifont.otf",size);if(!f)continue;
  hb_font_t* hbfont=hb_ft_font_create_referenced(f);hb_buffer_t* buffer=hb_buffer_create();
  hb_buffer_add_utf32(buffer,chars.data(),chars.size(),run.start,run.end-run.start);
  hb_buffer_set_direction(buffer,run.level%2?HB_DIRECTION_RTL:HB_DIRECTION_LTR);hb_buffer_set_script(buffer,run.script);hb_buffer_set_language(buffer,hb_language_from_string("und",-1));hb_shape(hbfont,buffer,nullptr,0);
  auto* infos=hb_buffer_get_glyph_infos(buffer,&count);auto* positions=hb_buffer_get_glyph_positions(buffer,&count);
  for(unsigned i=0;i<count;++i){
   unsigned id=infos[i].codepoint;
   if(id==0)id=FT_Get_Char_Index(f,chars[infos[i].cluster]);
   if(id&&!FT_Load_Glyph(f,id,FT_LOAD_DEFAULT)){
    int x=pen+positions[i].x_offset/64+f->glyph->metrics.horiBearingX/64;
    int y=size-positions[i].y_offset/64-f->glyph->metrics.horiBearingY/64;
    glyphs.push_back({f,id,pen+positions[i].x_offset/64,size-positions[i].y_offset/64});
    minY=std::min(minY,y);maxY=std::max(maxY,y+int(f->glyph->metrics.height/64));minX=std::min(minX,x);maxX=std::max(maxX,x+int(f->glyph->metrics.width/64));
   }
   pen+=positions[i].x_advance/64;
  }
  hb_buffer_destroy(buffer);hb_font_destroy(hbfont);
 }
 int w=std::max(1,std::min(maxWidth,std::max(pen,maxX)-minX+2));int h=std::min(96,std::max(1,maxY-minY+2));
 auto* pixels=static_cast<unsigned char*>(calloc(size_t(w)*h,4));if(!pixels)return nullptr;
 for(auto g:glyphs){if(FT_Load_Glyph(g.face,g.id,FT_LOAD_DEFAULT)||FT_Render_Glyph(g.face->glyph,FT_RENDER_MODE_NORMAL))continue;
  const auto& bitmap=g.face->glyph->bitmap;int x=g.x+g.face->glyph->bitmap_left-minX+1,y=g.y-g.face->glyph->bitmap_top-minY+1;
  for(unsigned by=0;by<bitmap.rows;++by)for(unsigned bx=0;bx<bitmap.width;++bx){int px=x+bx,py=y+by;if(px<0||py<0||px>=w||py>=h)continue;
   unsigned char alpha=bitmap.pixel_mode==FT_PIXEL_MODE_MONO?((bitmap.buffer[by*bitmap.pitch+bx/8]&(0x80>>(bx%8)))?255:0):bitmap.buffer[by*bitmap.pitch+bx];
   size_t index=(size_t(py)*w+px)*4;pixels[index]=pixels[index+1]=pixels[index+2]=255;pixels[index+3]=std::max(pixels[index+3],alpha);
  }
 }
 *width=w;*height=h;return pixels;
}
