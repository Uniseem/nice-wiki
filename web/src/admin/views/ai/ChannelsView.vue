<script setup lang="ts">
import{onMounted,reactive,ref}from'vue';import PageHeader from'../../components/PageHeader.vue';import{client,errorMessage,requireData}from'../../../shared/api/client';import type{components}from'../../../shared/api/schema.gen';
const channels=ref<components['schemas']['Channel'][]>([]);const models=ref<components['schemas']['ModelInfo'][]>([]);const editing=ref('');const headers=ref('');const form=reactive<components['schemas']['ChannelInput']>({name:'',provider:'',baseURL:'',protocol:'openai',apiKey:'',enabled:true,timeoutSeconds:90,streamTimeoutSeconds:30,concurrency:4});const error=ref('');
const model=reactive<components['schemas']['ModelInfo']>({id:'',name:'',channel:'',capabilities:['chat'],inputPrice:0,outputPrice:0,context:128000,maxOutput:4096});
async function refresh(){channels.value=requireData(await client.GET('/api/admin/models/channels'));models.value=requireData(await client.GET('/api/admin/models/catalog'))}
onMounted(async()=>{try{await refresh()}catch(cause){error.value=errorMessage(cause)}})
function edit(value:components['schemas']['Channel']){editing.value=value.id;headers.value='';Object.assign(form,{name:value.name,provider:value.provider,baseURL:value.baseURL,protocol:value.protocol,apiKey:'',enabled:value.enabled,timeoutSeconds:value.timeoutSeconds,streamTimeoutSeconds:value.streamTimeoutSeconds,concurrency:value.concurrency})}
function reset(){editing.value='';headers.value='';Object.assign(form,{name:'',provider:'',baseURL:'',protocol:'openai',apiKey:'',enabled:true,timeoutSeconds:90,streamTimeoutSeconds:30,concurrency:4})}
async function save(){try{const body:components['schemas']['ChannelInput']={...form};if(headers.value.trim())body.headers=JSON.parse(headers.value) as Record<string,string>;if(editing.value)requireData(await client.PUT('/api/admin/models/channels/{id}',{params:{path:{id:editing.value}},body}));else requireData(await client.POST('/api/admin/models/channels',{body}));reset();await refresh()}catch(cause){error.value=errorMessage(cause)}}
async function remove(id:string){if(!confirm('删除模型渠道？使用该渠道的模型分配将不可用。'))return;try{requireData(await client.DELETE('/api/admin/models/channels/{id}',{params:{path:{id}}}));await refresh()}catch(cause){error.value=errorMessage(cause)}}
async function probe(id:string){const modelId=prompt('用于连接探测的对话模型 ID');if(!modelId)return;try{requireData(await client.POST('/api/admin/models/channels/{id}/probe',{params:{path:{id}},body:{model:modelId}}));alert('连接正常')}catch(cause){error.value=errorMessage(cause)}}
async function saveModel(){try{requireData(await client.PUT('/api/admin/models/catalog',{body:model}));Object.assign(model,{id:'',name:'',channel:'',capabilities:['chat'],inputPrice:0,outputPrice:0,context:128000,maxOutput:4096});await refresh()}catch(cause){error.value=errorMessage(cause)}}
async function refreshCatalog(){try{requireData(await client.POST('/api/admin/models/catalog/refresh'));alert('已投递模型目录刷新任务')}catch(cause){error.value=errorMessage(cause)}}
</script>
<template>
  <PageHeader
    title="模型渠道与目录"
    description="渠道密钥不会回传；provider 填 models.dev 的提供方 ID，可复用内置模型资料。"
  /><p
    v-if="error"
    class="text-danger"
  >
    {{ error }}
  </p><form
    class="mb-8 grid gap-3 sm:grid-cols-2"
    @submit.prevent="save"
  >
    <input
      v-model="form.name"
      required
      class="nw-input"
      placeholder="渠道名称"
    ><input
      v-model="form.provider"
      class="nw-input"
      placeholder="models.dev provider ID（可选）"
    ><input
      v-model="form.baseURL"
      required
      type="url"
      class="nw-input"
      placeholder="接口根地址"
    ><select
      v-model="form.protocol"
      class="nw-input"
    >
      <option value="openai">
        OpenAI 兼容
      </option><option value="anthropic">
        Anthropic
      </option><option value="gemini">
        Gemini
      </option><option value="rerank">
        重排
      </option>
    </select><input
      v-model="form.apiKey"
      :required="!editing"
      type="password"
      class="nw-input"
      placeholder="密钥；留空表示不修改"
    ><label class="flex gap-2"><input
      v-model="form.enabled"
      type="checkbox"
    >启用渠道</label><label>普通超时秒数<input
      v-model.number="form.timeoutSeconds"
      type="number"
      min="0"
      max="600"
      class="nw-input"
    ></label><label>流式片段超时秒数<input
      v-model.number="form.streamTimeoutSeconds"
      type="number"
      min="0"
      max="600"
      class="nw-input"
    ></label><label>并发上限<input
      v-model.number="form.concurrency"
      type="number"
      min="0"
      max="128"
      class="nw-input"
    ></label><label class="sm:col-span-2">附加请求头 JSON<textarea
      v-model="headers"
      rows="4"
      class="nw-input"
      placeholder="留空表示不修改；例如：{&quot;X-Organization&quot;:&quot;demo&quot;}"
    /></label><div class="flex gap-3">
      <button class="nw-button">
        {{ editing?'更新':'新增' }}渠道
      </button><button
        v-if="editing"
        type="button"
        @click="reset"
      >
        取消
      </button>
    </div>
  </form><h2 class="text-xl font-semibold">
    渠道
  </h2><article
    v-for="value in channels"
    :key="value.id"
    class="mt-2 flex items-center gap-3"
  >
    <span class="grow">{{ value.name }} · {{ value.protocol }} · {{ value.enabled?'已启用':'已停用' }}<small
      v-if="value.headerNames.length"
      class="block text-muted"
    >附加头：{{ value.headerNames.join('、') }}</small></span><button @click="probe(value.id)">
      测试连接
    </button><button @click="edit(value)">
      编辑
    </button><button
      class="text-danger"
      @click="remove(value.id)"
    >
      删除
    </button>
  </article><h2 class="mt-8 text-xl font-semibold">
    模型目录
  </h2><button
    type="button"
    class="mt-3"
    @click="refreshCatalog"
  >
    从 models.dev 刷新目录
  </button><form
    class="my-4 grid gap-2 sm:grid-cols-4"
    @submit.prevent="saveModel"
  >
    <input
      v-model="model.channel"
      required
      class="nw-input"
      placeholder="渠道 ID"
    ><input
      v-model="model.id"
      required
      class="nw-input"
      placeholder="模型 ID"
    ><input
      v-model="model.name"
      required
      class="nw-input"
      placeholder="显示名称"
    ><select
      v-model="model.capabilities"
      multiple
      class="nw-input"
    >
      <option value="chat">
        对话
      </option><option value="vision">
        视觉
      </option><option value="structured">
        结构化
      </option><option value="embedding">
        嵌入
      </option><option value="rerank">
        重排
      </option>
    </select><input
      v-model.number="model.inputPrice"
      type="number"
      min="0"
      step="0.0001"
      class="nw-input"
      placeholder="输入单价"
    ><input
      v-model.number="model.outputPrice"
      type="number"
      min="0"
      step="0.0001"
      class="nw-input"
      placeholder="输出单价"
    ><input
      v-model.number="model.context"
      type="number"
      class="nw-input"
      placeholder="上下文"
    ><input
      v-model.number="model.maxOutput"
      type="number"
      class="nw-input"
      placeholder="最大输出"
    ><button class="nw-button">
      保存模型
    </button>
  </form><ul>
    <li
      v-for="value in models"
      :key="`${value.channel}:${value.id}`"
    >
      {{ value.name }} · {{ value.channel }} · {{ value.capabilities.join('、') }}
    </li>
  </ul>
</template>
