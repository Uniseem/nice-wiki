# 私有备份分片容器 V1

这是本域私有at-rest格式，不改变便携ZIP无需源key的产品行为。使用Go标准库AES-256-GCM，每对象随机32字节DEK；可信keyring把keyId映到启动配置的32字节本地KEK。keyId只作指纹，不是密钥，不接受恢复输入提供文件路径。包装DEK仅含随机DEK，不含业务正文或凭据。

文件依次为8字节ASCII `NWBAE001`、uint32大端header字节数、canonical UTF8 JSON CryptoHeaderV1、若干数据帧、唯一终帧和EOF。header最大16384字节，meta.format=nice-wiki.backup.chunked、schemaVersion=1、chunkSize=1048576，sourceBytes最大20GiB。JSON必须与本域canonical编码逐字节一致，拒绝重复/未知字段、非canonical及尾随内容。

header.meta的backupId/objectId/owner/objectKind/keyId/sourceHash/sourceBytes/dataSchemaVersion及随机8字节noncePrefix均参与KEK包装AAD：ASCII域`nice-wiki.backup.dek.v1`后一个零字节，再接canonical meta。wrappedNonce是随机12字节，wrappedDEK是GCM封装32字节DEK后的48字节；这三个二进制字段均标准base64。每个已安装对象只对应一次独立DEK；重放已完成原操作使用已校验的相同密文。未安装失败尝试只能清自己的临时密文，并使用新objectId/DEK重试。

每帧头为kind(uint8)、sequence(uint32大端)、plaintextLength(uint32大端)，随后恰好plaintextLength+16字节ciphertext。数据帧kind=1，sequence从0连续；长度必须是剩余明文与1MiB的较小值，不得有空数据帧。帧nonce=noncePrefix的8字节拼接sequence的4字节大端；独立DEK及有界单调计数保证不复用。帧AAD为ASCII域`nice-wiki.backup.frame.v1`后一个零字节、完整header的32字节SHA256、9字节帧头，绑定所有源事实及片序。

终帧kind=2，sequence=数据片数，plaintextLength=44。其明文依次uint64总明文长、uint32数据片数、32字节明文SHA256；以该sequence的独立nonce和同一帧AAD加密。空源也必须有终帧。数据总长/源摘要/终帧摘要与片数都必须一致，终帧后必须立即EOF；缺片、乱序、重复终帧、截断、尾随、错误key或任意tag失败均不激活。

每片1MiB，持有固定数量片缓冲（约2MiB及少量头部），不将20GiB对象载入内存。解密明文只落本域restore隔离临时文件，全部认证和EOF完成后fsync、rename、fsync目录再交付路径。加密同样先写临时文件并fsync，控制库FULL同步journal先记原输入摘要/prepared和完整密文引用，再安装文件/同步目录、记object-installed、提交active对象索引、记complete及原结果。控制库不随active替换，不能省略原输入摘要核对或把SQLite事务当跨文件原子提交。

布局V2为dataRoot/vault/<objectId>.nwb，temp为.<objectId>.tmp；索引及备份权威在control，不随active切换。容器V1字节不变。加密幂等范围包括backupId/owner/role/keyId，同范围异原输入CONFLICT。
