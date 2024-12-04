import csv
import random
import matplotlib.pyplot as plt

# numbers = [100, 300, 500, 700, 900, 1100]
# weights = [0.1, 0.2, 0.3, 0.2, 0.1, 0.1]  # 对应每个数的选择概率
# sequence = random.choices(numbers, weights=weights, k=100)
def generate_number_list(number_list, num_60, num_40, final_length):
    result = []
    for i in range(final_length):
        if i % num_60 < num_40:
            result.append(number_list[0])
        else:
            result.append(number_list[1])
    return result

number_list = [10, 100]
length = 40
length1 = 30
length2 = 10
final_length = 3000

sequence = generate_number_list(number_list, length, length1,final_length)
print(sequence)

with open('sequence.csv', 'w', newline='') as csvfile:
    writer = csv.writer(csvfile)
    for i, num in enumerate(sequence):
        writer.writerow([num])

plt.plot(sequence, label='actual')

# 添加图例、标题和轴标签
plt.legend()
plt.title('view-change-number'+str(num))
plt.xlabel('Block Index')
plt.ylabel('time(s)')

# 显示图形
plt.show()

# [0.672707608,0.523881347,0.504803549,0.487356358,0.461515035,0.455043429]
# [0.581463036,0.570267637,0.566369647,0.534597803,0.542238614,0.521732936]
#
# [0.60489033,0.441332299,0.491938754,0.441983676,0.433516335,0.477978025]
# [0.666650737,0.57097563,0.653203956,0.56219736,0.563204266,0.608538221]






