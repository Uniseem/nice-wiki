import { createInterface } from 'node:readline';
import { AgentRuntime } from './upstream/core/src/runtime/agent-runtime.ts';
import { InMemorySessionEventStore, createSessionId, modelMessageContentToText, getCurrentModelInvocationContext } from './upstream/contracts/src/index.ts';
import { createToolRegistry } from './upstream/core/src/tool/registry.ts';
import { ContextBuilder } from './upstream/core/src/context/builder.ts';
import { release } from 'node:os';

// 这里只连接 ZCode 的宿主端口。模型循环、工具调度、窗口与压缩均使用上游 AgentRuntime。
const pending = new Map();
const active = new Map();
let sequence = 0;
const emit = frame => process.stdout.write(JSON.stringify(frame) + '\n');

function rpc(runId, kind, payload, signal) {
  const id = String(++sequence);
  return new Promise((resolve, reject) => {
    const cancel = () => { pending.delete(id); reject(new DOMException('调用已取消', 'AbortError')); };
    if (signal?.aborted) { cancel(); return; }
    signal?.addEventListener('abort', cancel, {once:true});
    pending.set(id, {
      resolve(value) { signal?.removeEventListener('abort', cancel); resolve(value); },
      reject(error) { signal?.removeEventListener('abort', cancel); reject(error); },
    });
    emit({runId, kind, id, payload:JSON.stringify(payload)});
  });
}

const toNative = message => ({
  role:message.role, content:message.content,
  ...(message.toolCalls?.length ? {toolCalls:message.toolCalls.map(call => ({id:call.id,name:call.name,input:JSON.parse(call.arguments)}))} : {}),
  ...(message.toolCallId ? {toolCallId:message.toolCallId} : {}),
});

const toGateway = message => ({
  role:message.role, content:modelMessageContentToText(message.content),
  ...(message.toolCalls?.length ? {toolCalls:message.toolCalls.map(call => ({id:call.id,name:call.name,arguments:JSON.stringify(call.input)}))} : {}),
  ...(message.toolCallId ? {toolCallId:message.toolCallId} : {}),
});

async function run(runId, input) {
  const controller = new AbortController();
  active.set(runId, controller);
  let runtime;
  try {
    const state = input.state ? JSON.parse(input.state) : undefined;
    const signatures = new Map(Object.entries(state?.signatures || {}));
    for (const message of input.messages) for (const call of message.toolCalls || []) if (call.signature) signatures.set(call.id, call.signature);
    const project = message => {
      const value = toGateway(message);
      for (const call of value.toolCalls || []) if (signatures.has(call.id)) call.signature = signatures.get(call.id);
      return value;
    };
    const registry = createToolRegistry();
    for (const tool of input.tools) {
      registry.register({
        inputSchema:tool.parameters,
        metadata:{name:tool.name,description:tool.description,readOnly:tool.name !== 'report_abuse',destructive:false,concurrentSafe:tool.name !== 'report_abuse',needsApproval:false,riskLevel:'low',sideEffectScope:tool.name === 'report_abuse' ? 'network' : 'none'},
        handler:async (argumentsValue, context) => {
          const result = await rpc(runId, 'tool', {
            call:{id:context.toolCallId,name:tool.name,arguments:JSON.stringify(argumentsValue)},
            // 文档按页交付，单条结果符合网关的 64KiB 消息传输边界。
            maximumBytes:Math.max(4096, Math.min(60000, input.model.contextWindow * 2)),
          }, context.abortSignal);
          if (result.isError) throw new Error(result.content);
          return result.content;
        },
      });
    }

    const properties = {
      contextWindow:input.model.contextWindow,
      inputFormat:{supportsText:true,supportsImage:false,supportsVideo:false,supportsAudio:false,supportsPdf:false},
      outputFormat:{supportsText:true},supportsToolCall:true,supportsJsonSchemaOutput:false,
      supportsNativeWebSearch:false,supportsMidConversationSystem:true,requiresMfjsToolSchema:false,
    };
    const optionSpecs = {reasoningLevel:{values:['disabled']},maxOutputTokens:{max:input.model.maxOutputTokens}};
    function model(options = {}) {
      const invoke = async request => {
        const result = await rpc(runId, 'invoke', {
          public:getCurrentModelInvocationContext()?.modelCall?.operation === 'agent_step',
          messages:request.messages.map(project),
          tools:request.tools?.map(tool => ({name:tool.name,description:tool.description || '',parameters:tool.inputSchema})),
          maximumOutputTokens:request.options?.maxOutputTokens || options.maxOutputTokens || input.model.maxOutputTokens,
          toolChoice:request.tools?.length ? 'auto' : 'none',
        }, request.abortSignal);
        for (const call of result.output.toolCalls || []) if (call.signature) signatures.set(call.id, call.signature);
        if (result.model) {
          input.model = result.model;
          properties.contextWindow = result.model.contextWindow;
          optionSpecs.maxOutputTokens.max = result.model.maxOutputTokens;
        }
        return {
          text:result.output.text || '',
          finishReason:result.output.finishReason === 'tool_calls' ? 'tool-calls' : result.output.finishReason || 'stop',
          toolCalls:result.output.toolCalls?.map(call => ({id:call.id,name:call.name,input:JSON.parse(call.arguments)})),
          usage:result.usage || result.output.usage || {},
        };
      };
      return {
        get providerId() { return input.model.providerId; }, get modelId() { return input.model.modelId; }, properties,
        optionSpecs,
        options,
        bind(next) { return model({...options,...next}); },
        generateText:invoke,
        async *streamText(request) {
          const result = await invoke(request);
          if (result.text) {
            yield {type:'text_start',id:'answer'};
            yield {type:'text_delta',id:'answer',text:result.text};
            yield {type:'text_end',id:'answer'};
          }
          for (const toolCall of result.toolCalls || []) yield {type:'tool_call',toolCall};
          yield {type:'finish',finishReason:result.finishReason,usage:result.usage};
        },
      };
    }

    const messages = input.messages.map(toNative);
    const contextBuilder = new ContextBuilder({
      workingDirectory:process.cwd(),customSystemPrompt:messages[0]?.content,
      envInfo:{cwd:process.cwd(),platform:process.platform,shell:'',osVersion:release(),nodeVersion:process.version,isGitRepository:false},
    });
    runtime = new AgentRuntime(createSessionId(), {
      modelSelection:{providerId:input.model.providerId,modelId:input.model.modelId},
      workingDirectory:process.cwd(),systemPrompt:messages[0]?.content,language:input.language,
      mode:'yolo',toolAllowlist:input.tools.map(tool => tool.name),dynamicWorkflowEnabled:false,
      subagents:{enabled:false},titleGeneration:{enabled:false},
      compact:{enabled:true,get bufferTokens() { return Math.min(13000,Math.floor(input.model.contextWindow * .02)); }},
    }, {
      eventStore:new InMemorySessionEventStore(),toolRegistry:registry,contextBuilder,modelFactory:() => model(),
      eventSink:{onSessionEvent(event) {
        if (event.type.startsWith('compact_')) emit({runId,kind:'event',payload:JSON.stringify({type:event.type})});
      }},
    });

    // 上游导出的运行时消息保存入口；检查点只由 owning-service 保存和复核，不接收访客提供的角色。
    if (state?.schemaVersion === 1) {
      runtime.messageHistory.init(state.entries);
      const entries = runtime.messageHistory.toRuntimeEntries();
      const system = entries.find(entry => entry.message?.role === 'system');
      if (system) system.message.content = messages[0].content;
      runtime.messageHistory.init(entries);
      runtime.messageHistory.addEntries(messages.slice(1, -1).map(message => ({message})));
    } else runtime.messageHistory.init(messages.slice(0, -1));

    const result = await runtime.executeTurn(modelMessageContentToText(messages.at(-1).content), undefined, {abortSignal:controller.signal});
    const entries = runtime.messageHistory.toRuntimeEntries();
    const kept = new Set(entries.flatMap(entry => entry.message?.toolCalls?.map(call => call.id) || []));
    const checkpoint = JSON.stringify({schemaVersion:1,entries,signatures:Object.fromEntries([...signatures].filter(([id]) => kept.has(id)))});
    emit({runId,kind:'done',payload:JSON.stringify({text:result.response,state:checkpoint})});
  } catch (error) {
    if (process.env.NW_DEBUG) console.error(error?.code || error?.name, error?.stack?.split('\n').slice(1, 4).join('\n'));
    emit({runId,kind:'failed',payload:JSON.stringify({code:controller.signal.aborted ? 'CANCELLED' : 'DEPENDENCY_UNAVAILABLE'})});
  } finally {
    runtime?.beginShutdown();
    active.delete(runId);
  }
}

const lines = createInterface({input:process.stdin,crlfDelay:Infinity});
lines.on('line', line => {
  const frame = JSON.parse(line);
  if (frame.kind === 'start') void run(frame.runId, JSON.parse(frame.payload));
  else if (frame.kind === 'cancel') active.get(frame.runId)?.abort();
  else if (frame.kind === 'reply') {
    const request = pending.get(frame.id);
    pending.delete(frame.id);
    if (!request) return;
    const value = JSON.parse(frame.payload);
    if (value.error) request.reject(new Error(value.error));
    else request.resolve(value);
  }
});
lines.on('close', () => { for (const controller of active.values()) controller.abort(); });
