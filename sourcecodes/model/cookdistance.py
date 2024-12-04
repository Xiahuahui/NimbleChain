import numpy as np

class DataPoint:
    def __init__(self, data, delay):
        self.data = data
        self.delay = delay

def calculate_cook_distance(group, coefficients):
    n = len(group)
    p = len(coefficients)

    # Fit the model and calculate the residuals
    X = np.column_stack((np.ones(n), group[:, 0]))
    y = group[:, 0]
    b = np.linalg.inv(X.T @ X) @ X.T @ y
    y_pared = X @ b
    residuals = y - y_pared
    rss = np.sum(residuals ** 2)

    # Calculate the hat matrix
    H = X @ np.linalg.inv(X.T @ X) @ X.T

    # Calculate Cook's distance
    cook_d = np.zeros(n)
    for i in range(n):
        e_i = np.zeros_like(residuals)
        e_i[i] = residuals[i]
        rss_e = np.sum((y - y_pared - e_i) ** 2)
        # 检查除以零的情况
        if rss == 0 or n - p - 1 <= 0:
            cook_d[i] = 0  # 设置为0，表示没有影响
        else:
            cook_d[i] = (rss_e - rss) / (rss * (n - p - 1)) + (residuals[i] ** 2 / rss)

    return cook_d

def process_data(input_file, output_file, total_data, group_size, a):
    data_points = []
    with open(input_file, 'r') as file:
        for line in file:
            data, delay = map(float, line.strip().split())
            data_points.append(DataPoint(data, delay))

    # Ensure we have the correct number of data points
    data_points = data_points[:total_data]

    # Convert to numpy array for easier manipulation
    data_array = np.array([(dp.data, dp.delay) for dp in data_points])

    # Calculate the number of full groups and the remaining data points
    num_full_groups = len(data_array) // group_size
    remaining_data_points = len(data_array) % group_size

    # Example coefficients, you should replace these with your actual coefficients
    coefficients = [1.0, 0.5]

    with open(output_file, 'w') as file:
        # 计算 top_a_percent，并确保至少为1
        top_a_percent = max(1, int(np.ceil(a / 100.0 * group_size)))
        output_data_set = set()
        delay_greater_than_zero_data_set = set()
        detected_outliers = []  # 用于存储检测到的异常点索引

        # Process full groups
        for i in range(num_full_groups):
            start_index = i * group_size
            end_index = start_index + group_size
            subgroup = data_array[start_index:end_index]
            cook_distances = calculate_cook_distance(subgroup, coefficients)

            # Sort the cook distances in descending order
            sorted_indices = np.argsort(cook_distances)[::-1]

            # 找到按照cook距离排序的前a%
            for k in range(top_a_percent):
                index = sorted_indices[k]
                output_data_set.add(subgroup[index, 0])
                detected_outliers.append(start_index + index)  # 记录全局索引

        # Process remaining data points if any
        if remaining_data_points > 0:
            start_index = num_full_groups * group_size
            subgroup = data_array[start_index:]
            cook_distances = calculate_cook_distance(subgroup, coefficients)

            # Sort the cook distances in descending order
            sorted_indices = np.argsort(cook_distances)[::-1]

            # 找到按照cook距离排序的前a%，但不超过剩余数据点的数量
            for k in range(min(top_a_percent, len(subgroup))):
                index = sorted_indices[k]
                output_data_set.add(subgroup[index, 0])
                detected_outliers.append(start_index + index)  # 记录全局索引

        # 记录实际上的异常点
        count_delay_greater_than_zero = sum(1 for _, delay in data_array if delay > 0)
        for index, (_, delay) in enumerate(data_array):
            if delay > 0:
                delay_greater_than_zero_data_set.add(index)

        # Find the intersection of the two sets
        intersection = output_data_set & delay_greater_than_zero_data_set

        # Print results for current a value
        file.write(f"a = {a}%\n")
        sorted_detected_outliers = sorted(detected_outliers)
        file.write(f"Detected outliers indices: {','.join(map(str, sorted_detected_outliers))}\n")
        recall = len(intersection) / count_delay_greater_than_zero if count_delay_greater_than_zero > 0 else 0
        file.write(f"Recall: {recall}\n")
        precision = len(intersection) / len(output_data_set) if len(output_data_set) > 0 else 0
        file.write(f"Precision: {precision}\n\n")

def cook(list1, list2, a, group_size):
    # 确保两个列表长度相同
    if len(list1) != len(list2):
        raise ValueError("Both lists must have the same number of elements.")

    # 创建要写入文件的内容
    with open('input_cook.txt', 'w') as file:
        # 写入a和b作为文件的前两行


        # 将两个列表作为两列写入文件
        for item1, item2 in zip(list1, list2):
            file.write(f"{item1} {item2}\n")

    # 计算文件的行数
    with open('input_cook.txt', 'r') as file:
        #使用count()函数计算行数，包括a和b的行
        total_data = sum(1 for line in file)

    process_data('input_cook.txt', 'output_cook.txt', total_data, group_size, a)






bct_list =[3,3,3,8]
label_list=[0, 0, 0, 1]
group_size=4
a=30
if __name__ == '__main__':
    cook(bct_list,label_list, a, group_size)
#调用cook函数即可完成，共四个参数，分别是传入的bct_list,label_list,自己设置的阈值a,和宽度group_size
#结果会生成在output_cook.txt里面，不用看input_cook.txt(这个是程序运行自动生成的)





