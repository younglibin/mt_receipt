安装 docker  ， 打包 docker：docker build -t bx_mt_project .

第⼀步：在 bx_mt_project 镜像右侧，点击"Run"（运⾏）按钮
<img width="2752" height="1048" alt="image" src="https://github.com/user-attachments/assets/c51e292f-cf0c-4da0-9014-3d7050038190" />

第⼆步：在弹出的"Run a container"（运⾏容器）对话框中，您需要配置以下选项：
<img width="2696" height="1234" alt="image" src="https://github.com/user-attachments/assets/19d3ac6f-7371-4ea6-9857-7a2132dfd642" />

Container name（容器名称）：可以⾃定义⼀个名称，如 bx_mt_project_container
Volumes（卷）：点击"Add volume"（添加卷）按钮，添加两个卷：
第⼀个卷：
Host Path（主机路径）：选择您本地存放MT.xlsx和receipt.xls⽂件的⽬录
（如 /Users/yourusername/File ）
Container Path（容器路径）：设置为 /app/File
第⼆个卷：
Host Path（主机路径）：选择您本地存放输出⽂件的⽬录
（如 /Users/yourusername/output ）
Container Path（容器路径）：设置为 /app/output
<img width="2598" height="1472" alt="image" src="https://github.com/user-attachments/assets/0b926b96-7f2a-4d6c-a485-9994863c08c2" />

第三步： 将要计算的⽂件上传到 File ⽬录下
点击 ： run，在output 下得到：
以后执⾏：
第⼀步：更新 本地⽬录中 File 下的MT ⽂件 和 receipt ⽂件 为本次要计算的⽂件；
找到containers 找到上次执⾏的配置， 点击 后边 执⾏按钮 即可， 就会在 output 下找到 计算结果。
