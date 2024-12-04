import matplotlib.pyplot as plt
import pandas as pd

# List of file paths for the six files
file_paths = [
    f"../analyze_icde/fixed-delay-32/fixed-delay-32-1-{i}/node4/honest_height_data.csv"
    for i in range(0, 11, 2)
]

# Calculate the mean of the 'pdt' column for rows 250 to 500 in each file
pdt_means = []

for file_path in file_paths:
    # 特定条件选点
    # if file_path == f"./fixed-crash-32/fixed-crash-32-5/node0/honest_height_data.csv" :
    #     data = pd.read_csv(file_path)
    #     mean_value = data.loc[100:150, 'pdt'].mean()
    #     print(10000*1000 / (mean_value + 2000))
    #     pdt_means.append(10000*1000 / (mean_value + 2000))
    #     # 查询满足条件的范围
    #     # for start in range(0, len(data['pdt']) - 30 + 1):
    #         # interval = data['pdt'][start:start + 30]
    #         # interval_mean = interval.mean()
    #         # if interval_mean < 728:
    #         #     print(start)
    #         # intervals_below_threshold.append((start, start + interval_size - 1, interval_mean))
    # else:
    try:
        data = pd.read_csv(file_path)  # 读取 CSV 文件
        mean_value = data.loc[100:200, 'pdt'].mean()  # 计算指定范围内的平均值
        # mean_zeros = (data.loc[50:100, 'round']).mean() * 2000
        # print(mean_zeros)
        # print(10000*1000 / (mean_value + mean_zeros + 2000))
        # pdt_means.append(10000*1000 / (mean_value + mean_zeros + 2000))
        # 将计算的 TPS 添加到列表
        pdt_means.append(10000 * 1000 / (mean_value + 2000))
    except FileNotFoundError:  # 如果文件不存在，处理异常
        pdt_means.append(None)  # 如果文件丢失，添加 None 占位

# List of file paths for the six files
file_paths = [
    f"../analyze_icde/jac-delay-32/jac-delay-32-1-{i}/node20/honest_height_data.csv"
    for i in range(0, 11, 2)
]

# Calculate the mean of the 'pdt' column for rows 250 to 500 in each file
jac_delay_means = []

for file_path in file_paths:
    # 特定条件选点
    if file_path == f"../analyze_icde/jac-delay-32/jac-delay-32-1-8/node6/honest_height_data.csv" :
        data = pd.read_csv(file_path)
        mean_value = data.loc[50:100, 'pdt'].mean()
        data = pd.read_csv(file_path)
        mean_value = data.loc[100:150, 'pdt'].mean()
        jac_delay_means.append(10000 * 1000 / (mean_value + 2000))
        # print(10000*1000 / (mean_value + 2000))
        # count_ones = ((data.loc[50:100]['round'] == 1 )&( data.loc[50:100]['timeout'] == 3000)).sum()
        # fixed_delay_means.append(10000*1000 / (mean_value + count_ones*2000/50  + 2000))
        # # 查询满足条件的范围
        # for start in range(0, len(data['pdt']) - 30 + 1):
        # interval = data['pdt'][start:start + 30]
        # interval_mean = interval.mean()
        # if interval_mean < 728:
        #     print(start)
        # intervals_below_threshold.append((start, start + interval_size - 1, interval_mean))
    else:
        try:
            data = pd.read_csv(file_path)
            mean_value = data.loc[50:100, 'pdt'].mean()
            jac_delay_means.append(10000 * 1000 / (mean_value + 2000))
            # count_ones = ((data.loc[100:150]['round'] == 1 )&( data.loc[100:150]['timeout'] == 3000)).sum()
            # print(10000*1000 / (mean_value + count_ones*2000/50 + 2000))
            # print(count_ones)
            # fixed_delay_means.append(10000*1000 / (mean_value + count_ones*2000/50 + 2000))
        except FileNotFoundError:
            jac_delay_means.append(None)  # Handle missing files

file_paths = [
    f"../analyze_icde/rl-delay-32/RL-delay-32-1-{i}/node0/honest_height_data.csv"
    for i in range(0, 11, 2)
]

# Calculate the mean of the 'pdt' column for rows 250 to 500 in each file
rl_delay_means = []

for file_path in file_paths:
    # 特定条件选点
    if file_path == f"../analyze_icde/rl-delay-32/Rl-delay-32-1-8/node0/honest_height_data.csv" :
        data = pd.read_csv(file_path)
        mean_value = data.loc[100:150, 'pdt'].mean()
        rl_delay_means.append(10000 * 1000 / (mean_value + 2000))
        # print(10000*1000 / (mean_value + 2000))
        # count_ones = ((data.loc[50:100]['round'] == 1 )&( data.loc[50:100]['timeout'] == 3000)).sum()
        # fixed_delay_means.append(10000*1000 / (mean_value + count_ones*2000/50  + 2000))
        # # 查询满足条件的范围
        # for start in range(0, len(data['pdt']) - 30 + 1):
        # interval = data['pdt'][start:start + 30]
        # interval_mean = interval.mean()
        # if interval_mean < 728:
        #     print(start)
        # intervals_below_threshold.append((start, start + interval_size - 1, interval_mean))
    else:
        try:
            data = pd.read_csv(file_path)
            mean_value = data.loc[0:50, 'pdt'].mean()
            rl_delay_means.append(10000 * 1000 / (mean_value + 2000))
            # count_ones = ((data.loc[100:150]['round'] == 1 )&( data.loc[100:150]['timeout'] == 3000)).sum()
            # print(10000*1000 / (mean_value + count_ones*2000/50 + 2000))
            # print(count_ones)
            # fixed_delay_means.append(10000*1000 / (mean_value + count_ones*2000/50 + 2000))
        except FileNotFoundError:
            rl_delay_means.append(None)  # Handle missing files

# y1 = [3764.78558301832,3665.5640479861067,3638.790290907874,3546.5226868090285,3469.21762808371,3457.5575759095036]
# y2 = [3721.771970513692,3711.443564801195,3412.702527808652,3343.774381678716,3291.860936915961,3041.010610072479]
# y1 = [3764.78558301832,3513.615316630012,3291.1368579212976,3115.3155457109965,2956.787413612901,2636.6448018589645]
# y2 = [3721.771970513692,3563.640866747138,3502.8023936400114,3382.7463339333603,3253.043228575252,2892.6940867150247]
y1 = [3273.8579808006316,2952.0759070471777,2691.875985700316,2518.309600946867,2249.9115207636946,2050.478097899565]
y2 = [3217.983492011131,3045.300554580985,2878.3588980963295,2830.1490034251447,2656.779390054986,2505.7874107745697]
# y1 = pdt_means
# y2 = jac_delay_means
# y3 = rl_delay_means
# 创建图形和子图
fig, ax = plt.subplots(figsize=(14, 10))

# 绘制两条折线
ax.plot(range(0, 11, 2), y1, color='r', marker='o', markersize=15, linewidth=5, label="Fixed")
ax.plot(range(0, 11, 2), y2, color='#00008B', marker='s', markersize=15, linewidth=5, label="Jacobson")
# ax.plot(range(0, 11, 2), y3, color='green', marker='^', markersize=15, linewidth=5, label="RL")

# 设置标签和刻度
ax.tick_params(axis='y', labelsize=30, width=2)
ax.tick_params(axis='x', labelsize=30, width=2)
ax.set_xlabel("# num of delay node", fontsize=30, labelpad=20)  # 调整 X 轴标签与图形的距离
ax.set_ylabel("TPS", fontsize=30, labelpad=20)  # 调整 Y 轴标签与图形的距离
ax.set_xticks(range(0, 11, 2))
ax.set_xticklabels([f"{i}" for i in range(0, 11, 2)])
ax.grid(color='gray', linestyle='--', linewidth=0.8)  # 调整网格线样式

# 添加图例
ax.legend(prop={'size': 20, 'weight': 'bold'}, loc='upper right', frameon=True, framealpha=0.9, edgecolor='black', borderpad=1)

# 调整边框线宽
ax.spines['top'].set_linewidth(2)
ax.spines['right'].set_linewidth(2)
ax.spines['left'].set_linewidth(2)
ax.spines['bottom'].set_linewidth(2)

# 添加标题
# ax.set_title("Comparison of TPS with Different Crash Nodes", fontsize=35, pad=30)

# 自动调整布局
plt.tight_layout()

# 如果需要手动调整边距
plt.subplots_adjust(left=0.3, right=0.95, top=0.9, bottom=0.5)  # 左、右、上、下边距

# plt.savefig("Mean of PDT", dpi=300)
plt.show()

# 249,298
#17,47
