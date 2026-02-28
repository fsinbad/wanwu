from langchain.text_splitter import CharacterTextSplitter
import re
import os
from typing import List, Tuple


def process_string(long_str, punctuation_list, size):  
    # 存储逆序后的句子列表（包括标点）  
    sentences_reversed = []  
    current_sentence = ''  
      
    # 从后往前遍历字符串  
    for i in range(len(long_str) - 1, -1, -1):  
        if long_str[i] in punctuation_list:  
            # 如果当前字符是标点，则先添加到当前句子中  
            current_sentence += long_str[i]  
            # 然后检查是否要添加这个完整的句子到列表中  
            if current_sentence:  
                sentences_reversed.append(current_sentence[::-1])  
                current_sentence = ''  # 重置当前句子  
        else:  
            # 否则，继续构建当前句子  
            current_sentence += long_str[i]  
      
    # 如果最后一个句子（可能没有标点）不为空，也添加到列表中  
    if current_sentence:  
        sentences_reversed.append(current_sentence[::-1])  
      
    # 构建最终的结果字符串，同时检查长度  
    final_str = ''  
    for sentence in sentences_reversed:  
        # 检查加上当前句子后是否超过长度限制  
        if len(final_str) + len(sentence) <= size:  
            final_str = sentence + final_str  
        else:  
            # 如果超过，则停止添加句子  
            break  
      
    return final_str  
  

def remove_leading_punctuation(s, punctuation_list):  

    # 遍历标点符号列表  

    for punct in punctuation_list:  

        # 如果字符串以该标点符号开始，则去除它  

        if s.startswith(punct):  

            return s[1:]  

    # 如果没有找到匹配的标点符号，则返回原始字符串  

    return s


def generate_regex(punc_list):
    escaped_punc = [re.escape(p) for p in punc_list]
    # return r'([' + ''.join(escaped_punc) + r'])'
    return re.compile('(' + '|'.join(escaped_punc) + ')')


def replace_k_consecutive_nl(separator: str, text: str) -> (str, str):
    normalized = separator.replace('\\n', '\n')
    k = len(normalized) if re.fullmatch(r'\n+', normalized) else 0
    if k == 0:
        return separator, text

    pattern = re.compile(rf'\n{{{k}}}')
    return '<NLS>', pattern.sub('<NLS>', text)

IMG_PATTERN = r'!\[[^\]]*\]\([^\s)]+(?:\s+"[^"]*")?\)'

def protect_images(text: str) -> Tuple[str, List[str]]:
    """将图片标签替换为占位符，并返回占位符列表"""
    images = re.findall(IMG_PATTERN, text)
    for i, img in enumerate(images):
        # 使用特殊标记替换，例如 <IMG_PROTECT_0>
        text = text.replace(img, f"<IMG_PROTECT_{i}>", 1)
    return text, images


def restore_images(chunks: List[str], images: List[str]) -> List[str]:
    """将切分后的 chunk 中的占位符还原回图片标签"""
    restored_chunks = []
    for chunk in chunks:
        for i, img in enumerate(images):
            placeholder = f"<IMG_PROTECT_{i}>"
            if placeholder in chunk:
                chunk = chunk.replace(placeholder, img)
        restored_chunks.append(chunk)
    return restored_chunks


class ChineseTextSplitter(CharacterTextSplitter):
    def __init__(self, chunk_type: str = 'split_by_design', pdf: bool = False, excel: bool = False, sentence_size: int = 500, overlap_size: float = 0.0, separators: list = [], **kwargs):
        super().__init__(**kwargs)
        self.sentence_size = sentence_size
        self.chunk_type = chunk_type
        self.overlap_size = overlap_size if overlap_size else 0
        self.separators = separators if separators else ["。", "！", "？", ".", "!", "?", "……"]
        self.default_separators = ["。", "！", "？", ".", "!", "?", "……"]

    def split_text1(self, text: str) -> List[str]:
        # logger.info('走到通用切分')
        punctuation_list = self.separators
        def generate_regex(punc_list):
            escaped_punc = [re.escape(p) for p in punc_list]
            return r'([' + ''.join(escaped_punc) + r'])([^”’])'
        # 初始句子切分
        regex_replacements = [
            (generate_regex(punctuation_list), r"\1\n\2")
        ]
        for pattern, replacement in regex_replacements:
            text = re.sub(pattern, replacement, text)
        text = text.rstrip().replace(r'\u3000', ' ')
        sentences = [i for i in text.split("\n") if i]
    
        # 进一步切分长句子
        result_sentences = []
        for ele in sentences:
            if len(ele) > self.sentence_size:
                ele1 = re.sub(r'([.]["’”」』]{0,2})([^,，.])', r'\1\n\2', ele)
                ele1_ls = ele1.split("\n")
                for ele_ele1 in ele1_ls:
                    if len(ele_ele1) > self.sentence_size:
                        ele_ele2 = re.sub(r'([\n]{1,}| {2,}["’”」』]{0,2})([^\s])', r'\1\n\2', ele_ele1)
                        ele2_ls = ele_ele2.split("\n")
                        for ele_ele2 in ele2_ls:
                            if len(ele_ele2) > self.sentence_size:
                                ele_ele3 = re.sub(r'( ["’”」』]{0,2})([^ ])', r'\1\n\2', ele_ele2)
                                ele2_id = ele2_ls.index(ele_ele2)
                                ele2_ls = ele2_ls[:ele2_id] + [i for i in ele_ele3.split("\n") if i] + ele2_ls[ele2_id + 1:]
                        ele_id = ele1_ls.index(ele_ele1)
                        ele1_ls = ele1_ls[:ele_id] + [i for i in ele2_ls if i] + ele1_ls[ele_id + 1:]
                result_sentences.extend([i for i in ele1_ls if i])
            else:
                result_sentences.append(ele)
    
        sentences = result_sentences
    
        result = []
        if self.overlap_size <= 0:
            temp = ""
            for l in sentences:
                if len(l) < self.sentence_size:
                    temp += l
                else:
                    if temp != "":
                        result.append(temp)
                        temp = ""
                    result.append(l)
                    continue
                if len(temp) > self.sentence_size:
                    result.append(temp)
                    temp = ""
            if temp != "":
                result.append(temp)
        else:
            i = 0
            while i < len(sentences):
                # 计算当前段落
                temp = sentences[i]
                j = i + 1
                while j < len(sentences) and len(temp) + len(sentences[j]) <= self.sentence_size:
                    temp += sentences[j]
                    j += 1
                result.append(temp)
                
                # 计算重叠句子数量
                overlap_count = int(self.overlap_size * (j - i))
                if overlap_count < 1:
                    overlap_count = 1
                print('overlap_count',overlap_count)
                
                # 更新索引 i
                i = j - overlap_count if j - overlap_count > i else i + 1
            if len(result) == 0:
                print("列表为空")
            elif len(result[-1]) < self.sentence_size and len(result) > 1:
                result[-2] += result[-1]
                result = result[:-1]
    
        return result

    def split_text2(self, text: str) -> List[str]:
        # logger.info('走到自定义切分')
        punctuation_list = self.separators
        if not any(p in text for p in punctuation_list):
            punctuation_list = ["。", "！", "？", ".", "!", "?", "……"]

        def generate_regex(punc_list):
            escaped_punc = [re.escape(p) for p in punc_list]
            return r'([' + ''.join(escaped_punc) + r'])'

        regex_replacements = [
            (generate_regex(punctuation_list), r"\1\n")
        ]
        for pattern, replacement in regex_replacements:
            text = re.sub(pattern, replacement, text)
        text = text.rstrip().replace(r'\u3000', ' ')
        sentences = [i for i in text.split("\n") if i]

        result = []
        temp = ""
        overlap_length = 0
        overlap_content = ''
        
        for i in range(len(sentences)):
            sentence = sentences[i]
            temp += sentence
            if len(temp) <= self.sentence_size:
                temp = temp
            else:
                if i == 0:                 
                    a = temp[:self.sentence_size]
                    result.append(a)
                    temp = temp[self.sentence_size:]
                else:
                    a = temp[:-len(sentence)]
                    if len(a) >= self.sentence_size:
                        c = a[:self.sentence_size]
                        result.append(c)
                        temp = a[self.sentence_size:] + sentence
                    else:                        
                        result.append(a)
                        temp = sentence
                if self.overlap_size > 0:
                    overlap_content = process_string(a, self.separators, int(self.sentence_size * self.overlap_size)+1)
                    overlap_content = remove_leading_punctuation(overlap_content, self.separators)
                    temp = overlap_content+temp     
                else:
                    temp = temp
        
        while len(temp) >= self.sentence_size:
            result.append(temp[:self.sentence_size])
            temp = temp[self.sentence_size:]
        if temp: 
            result.append(temp)  
        return result

    def split_text3(self, text, separators) -> List[str]:
        # logger.info('走到自定义切分，支持正则表达式和自定义分割符')
        def split_with_regex(text, separator):
            splits = re.split(f"({re.escape(separator)})", text)
            results = [splits[i - 1] + splits[i] for i in range(1, len(splits), 2)]
            if len(splits) % 2 != 0:
                results += splits[-1:]
            return [t.rstrip().replace(r'\u3000', ' ') for t in results if (t not in {"", "\n"})]

        result = []
        separator = separators[-1]
        next_separators = []
        for i, sep in enumerate(separators):
            if sep == "":
                separator = sep
                break
            if re.search(sep, text):
                separator = sep
                next_separators = separators[i + 1:]
                break

        splits = split_with_regex(text, separator)
        temp = []
        for s in splits:
            if len(s) < self.sentence_size:
                temp.append(s)
            else:
                if temp:
                    merged_text = self.merge_splits(temp)
                    result.extend(merged_text)
                    temp = []
                if not next_separators:
                    result.append(s)
                else:
                    other_info = self.split_text3(s, next_separators)
                    result.extend(other_info)

        if temp:
            merged_text = self.merge_splits(temp)
            result.extend(merged_text)

        return result


    def merge_splits(self, splits) -> list[str]:
        merged_chunks = []
        temp_splits  = []
        total = 0
        overlap_chars = int(self.overlap_size * self.sentence_size)
        for split in splits:
            if len(temp_splits) > 0 and total + len(split)  > self.sentence_size:
                text = "".join(temp_splits).strip()
                if text != "" and text is not None:
                    merged_chunks.append(text)
                while total > overlap_chars or (total + len(split) > self.sentence_size and total > 0):
                    first = temp_splits[0]
                    if total - len(first) < overlap_chars and overlap_chars > 0:
                        need = overlap_chars - (total - len(first))
                        need_int = max(0, min(len(first), int(need)))
                        suffix = first[-need_int:]
                        temp_splits[0] = suffix
                        total = overlap_chars
                        break
                    else:
                        total -= len(first)
                        temp_splits = temp_splits[1:]
            temp_splits.append(split)
            total += len(split)
        text = "".join(temp_splits).strip()
        if text != "" and text is not None:
            merged_chunks.append(text)
        return merged_chunks


    def split_text_recursive(self, text: str, separators: List[str]) -> list[str]:
        finale_splits = []
        if not separators:
            splits = list(text)
            finale_splits = [s for s in splits if (s not in {"", "\n"})]
            return finale_splits

        separator = separators[0]
        new_separators = separators[1:]

        if separator == " ":
            splits = text.split()
        else:
            splits = text.split(separator)
            splits = [item + separator if index < (len(splits)-1) else item for index, item in enumerate(splits)]

        splits = [s for s in splits if (s not in {"", "\n"})]

        for split in splits:
            if len(split) < self.sentence_size:
                finale_splits.append(split)
            else:
                next_splits = self.split_text_recursive(split, new_separators)
                finale_splits.extend(next_splits)

        return finale_splits


    def split_text_by_hierarchy(self, text: str, separators: List[str]) -> list[str]:
        separator = ""
        new_separators = []
        if separators:
            separator = separators[0]
            new_separators = separators[1:]

        if separator:
            if separator == " ":
                splits = text.split()
            else:
                splits = text.split(separator)
                splits = [item + separator if index < (len(splits)-1) else item for index, item in enumerate(splits)]
        else:
            splits = list(text)
        splits = [s for s in splits if (s not in {"", "\n"})]
        final_chunks = []

        if separator != "":
            for split in splits:
                if len(split) < self.sentence_size:
                    final_chunks.append(split)
                else:
                    other_info = self.split_text_by_hierarchy(split, new_separators)
                    final_chunks.extend(other_info)
        else:
            current_text = ""
            total = 0
            overlap_text = ""
            overlap_text_length = 0
            for split_item in splits:
                split_item_length = len(split_item)
                if total + split_item_length >= self.sentence_size:
                    final_chunks.append(current_text)
                    current_text = overlap_text + split_item
                    total = split_item_length + overlap_text_length
                    overlap_text = ""
                    overlap_text_length = 0
                else:
                    current_text += split_item
                    total += split_item_length
                    if total > self.sentence_size - self.overlap_size:
                        overlap_text += split_item
                        overlap_text_length += split_item_length

            if current_text:
                final_chunks.append(current_text)

        return final_chunks


    def split_text_by_custom_separators(self, text: str) -> list[str]:
        new_separators = []
        for separator in self.separators:
            separator, text = replace_k_consecutive_nl(separator, text)
            if separator not in new_separators:
                new_separators.append(separator)
        # 如果分隔符里没有\n，先把原文中的\n替换为特殊标记
        if "\n" not in self.separators and "\\n" not in self.separators:
            text = text.replace("\n", "<NL>")
        regex_replacements = [
            (generate_regex(new_separators), r"\1\n")
        ]
        for pattern, replacement in regex_replacements:
            text = re.sub(pattern, replacement, text)
        text = text.rstrip().replace(r'\u3000', ' ')
        splits = [s for s in text.split("\n") if (s not in {"", "\n"})]

        final_chunks = []
        for s in splits:
            s= s.replace("<NLS>", "")
            s = s.replace("<NL>", "\n")
            if len(s) < self.sentence_size:
                final_chunks.append(s)
            else:
                temp_splits = self.split_text_recursive(s, self.default_separators)
                merged_chunks = self.merge_splits(temp_splits)
                final_chunks.extend(merged_chunks)

        return final_chunks


    def split_text(self, text: str) -> List[str]:
        # 保护markdown格式图片链接
        protected_text, saved_images = protect_images(text)

        if self.chunk_type == 'split_by_design':
            chunks = self.split_text_by_custom_separators(protected_text)
        else:
            chunks = self.split_text1(protected_text)

        # 还原图片
        return restore_images(chunks, saved_images)

if __name__ == "__main__":
    # question = input('Please enter your question: ')
    file_path = '/home/jovyan/RAG_2.0/langchain_rag_new/textsplitter/含有!的文本.txt'
    textsplitter = ChineseTextSplitter('split_by_design', sentence_size=600,overlap_size=0.25,separators=["!"])
    with open(file_path, 'r', encoding='utf-8') as file:
        content = file.read()    
    chunks = textsplitter.split_text(content)
    #print(chunks)
    for i, chunk in enumerate(chunks, start=1):
        chunk_len = len(chunk)
        # print(f"Chunk {i}:\n{chunk}\n{chunk_len}")
        print(f"Chunk {i}:{chunk_len}")
        print(f"Chunk {i}:{chunk}")
