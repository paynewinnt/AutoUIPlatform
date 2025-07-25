import React, { useEffect, useState } from 'react';
import {
  Card,
  Table,
  Button,
  Space,
  message,
  Tag,
  Typography,
  Row,
  Col,
  Statistic,
  Progress,
  DatePicker,
  Select,
  Popconfirm,
} from 'antd';
import dayjs from 'dayjs';
import {
  ReloadOutlined,
  DeleteOutlined,
  StopOutlined,
} from '@ant-design/icons';
import { api } from '../../services/api';
import type { TestExecution, Project, Environment } from '../../types';
import type { ColumnsType } from 'antd/es/table';

const { Title, Text } = Typography;
const { RangePicker } = DatePicker;
const { Option } = Select;

const Executions: React.FC = () => {
  const [executions, setExecutions] = useState<TestExecution[]>([]);
  const [projects, setProjects] = useState<Project[]>([]);
  const [environments, setEnvironments] = useState<Environment[]>([]);
  const [loading, setLoading] = useState(false);
  const [filters, setFilters] = useState<{
    project_id?: number;
    environment_id?: number;
    status?: string;
    execution_type?: string;
    date_range?: any;
  }>({
    project_id: undefined,
    environment_id: undefined,
    status: undefined,
    execution_type: undefined,
    date_range: null,
  });
  const [pagination, setPagination] = useState({
    current: 1,
    pageSize: 10,
    total: 0,
  });

  useEffect(() => {
    loadExecutions();
    loadInitialData();
  }, [pagination.current, pagination.pageSize, filters]);

  const loadInitialData = async () => {
    try {
      const [projectsData, environmentsData] = await Promise.all([
        api.getProjects({ page: 1, page_size: 100 }),
        api.getEnvironments(),
      ]);
      setProjects(projectsData.list);
      setEnvironments(environmentsData);
    } catch (error) {
      console.error('Failed to load initial data:', error);
    }
  };

  const loadExecutions = async () => {
    setLoading(true);
    try {
      const params: any = {
        page: pagination.current,
        page_size: pagination.pageSize,
      };

      if (filters.project_id) params.project_id = filters.project_id;
      if (filters.environment_id) params.environment_id = filters.environment_id;
      if (filters.status) params.status = filters.status;
      if (filters.execution_type) params.execution_type = filters.execution_type;
      if (filters.date_range && filters.date_range.length === 2 && filters.date_range[0] && filters.date_range[1]) {
        params.start_date = dayjs(filters.date_range[0]).format('YYYY-MM-DD');
        params.end_date = dayjs(filters.date_range[1]).format('YYYY-MM-DD');
      }

      const response = await api.getExecutions(params);
      setExecutions(response.list);
      setPagination(prev => ({
        ...prev,
        total: response.total,
      }));
    } catch (error) {
      console.error('Failed to load executions:', error);
      message.error('获取执行记录失败');
    } finally {
      setLoading(false);
    }
  };

  const handleDelete = async (id: number) => {
    try {
      await api.deleteExecution(id);
      message.success('删除成功');
      loadExecutions();
    } catch (error) {
      console.error('Failed to delete execution:', error);
      message.error('删除失败');
    }
  };

  const handleStop = async (id: number) => {
    try {
      // Assuming there's a stop execution API
      await api.stopExecution(id);
      message.success('停止成功');
      loadExecutions();
    } catch (error) {
      console.error('Failed to stop execution:', error);
      message.error('停止失败');
    }
  };

  const getStatusColor = (status: string) => {
    const colors: Record<string, string> = {
      success: 'green',
      failed: 'red',
      running: 'blue',
      pending: 'orange',
      cancelled: 'gray',
    };
    return colors[status] || 'default';
  };

  const getStatusText = (status: string) => {
    const texts: Record<string, string> = {
      success: '成功',
      failed: '失败',
      running: '运行中',
      pending: '等待中',
      cancelled: '已取消',
    };
    return texts[status] || status;
  };

  const calculateSuccessRate = (executions: TestExecution[]) => {
    if (executions.length === 0) return 0;
    const successCount = executions.filter(e => e.status === 'success').length;
    return Math.round((successCount / executions.length) * 100);
  };

  const calculateAverageDuration = (executions: TestExecution[]) => {
    const completedExecutions = executions.filter(e => e.duration > 0);
    if (completedExecutions.length === 0) return 0;
    const totalDuration = completedExecutions.reduce((sum, e) => sum + e.duration, 0);
    return Math.round(totalDuration / completedExecutions.length / 1000);
  };

  const columns: ColumnsType<TestExecution> = [
    {
      title: '执行ID',
      dataIndex: 'id',
      key: 'id',
      width: 80,
      sorter: (a, b) => a.id - b.id,
    },
    {
      title: '执行类型',
      dataIndex: 'execution_type',
      key: 'execution_type',
      width: 100,
      render: (type: string) => (
        <Tag color={type === 'test_case' ? 'blue' : 'green'}>
          {type === 'test_case' ? '测试用例' : '测试套件'}
        </Tag>
      ),
      filters: [
        { text: '测试用例', value: 'test_case' },
        { text: '测试套件', value: 'test_suite' },
      ],
      onFilter: (value, record) => record.execution_type === value,
    },
    {
      title: '名称',
      key: 'name',
      width: 200,
      ellipsis: true,
      render: (_, record) => (
        <div>
          <div>{record.test_case?.name || record.test_suite?.name}</div>
          <Text type="secondary" style={{ fontSize: '12px' }}>
            {record.test_case?.project?.name || record.test_suite?.project?.name}
          </Text>
        </div>
      ),
    },
    {
      title: '环境',
      key: 'environment',
      width: 100,
      render: (_, record) => (
        <Tag>
          {record.test_case?.environment?.name || record.test_suite?.environment?.name}
        </Tag>
      ),
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      width: 100,
      render: (status: string) => (
        <Tag color={getStatusColor(status)}>
          {getStatusText(status)}
        </Tag>
      ),
      filters: [
        { text: '成功', value: 'success' },
        { text: '失败', value: 'failed' },
        { text: '运行中', value: 'running' },
        { text: '等待中', value: 'pending' },
        { text: '已取消', value: 'cancelled' },
      ],
      onFilter: (value, record) => record.status === value,
    },
    {
      title: '测试结果',
      key: 'test_results',
      width: 120,
      render: (_, record) => (
        <div>
          <div>
            <Text style={{ color: '#52c41a' }}>✓ {record.passed_count}</Text>
            <Text style={{ margin: '0 4px' }}>/</Text>
            <Text style={{ color: '#ff4d4f' }}>✗ {record.failed_count}</Text>
            <Text style={{ margin: '0 4px' }}>/</Text>
            <Text>总 {record.total_count}</Text>
          </div>
          <Progress
            percent={record.total_count > 0 ? Math.round((record.passed_count / record.total_count) * 100) : 0}
            size="small"
            showInfo={false}
            strokeColor={record.passed_count === record.total_count ? '#52c41a' : '#ff4d4f'}
          />
        </div>
      ),
    },
    {
      title: '执行时长',
      dataIndex: 'duration',
      key: 'duration',
      width: 100,
      render: (duration: number) => {
        if (duration === 0) return '-';
        const seconds = Math.round(duration / 1000);
        if (seconds < 60) return `${seconds}s`;
        const minutes = Math.floor(seconds / 60);
        const remainingSeconds = seconds % 60;
        return `${minutes}m ${remainingSeconds}s`;
      },
      sorter: (a, b) => a.duration - b.duration,
    },
    {
      title: '开始时间',
      dataIndex: 'start_time',
      key: 'start_time',
      width: 150,
      render: (date: string) => new Date(date).toLocaleString(),
      sorter: (a, b) => new Date(a.start_time).getTime() - new Date(b.start_time).getTime(),
      defaultSortOrder: 'descend' as const,
    },
    {
      title: '结束时间',
      dataIndex: 'end_time',
      key: 'end_time',
      width: 150,
      render: (date: string | null) => date ? new Date(date).toLocaleString() : '-',
    },
    {
      title: '操作',
      key: 'action',
      width: 120,
      render: (_, record) => (
        <Space size="small">
          {record.status === 'running' && (
            <Popconfirm
              title="确定要停止这个执行吗？"
              onConfirm={() => handleStop(record.id)}
              okText="确定"
              cancelText="取消"
            >
              <Button
                type="link"
                size="small"
                danger
                icon={<StopOutlined />}
              >
                停止
              </Button>
            </Popconfirm>
          )}
          <Popconfirm
            title="确定删除这条执行记录吗？"
            onConfirm={() => handleDelete(record.id)}
            okText="确定"
            cancelText="取消"
          >
            <Button
              type="link"
              size="small"
              danger
              icon={<DeleteOutlined />}
              disabled={record.status === 'running'}
            >
              删除
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <div>
      <Title level={2}>执行记录</Title>
      
      <Row gutter={16} style={{ marginBottom: 24 }}>
        <Col span={6}>
          <Card>
            <Statistic 
              title="总执行次数" 
              value={pagination.total} 
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic
              title="成功率"
              value={calculateSuccessRate(executions)}
              suffix="%"
              valueStyle={{ 
                color: calculateSuccessRate(executions) >= 80 ? '#3f8600' : '#cf1322' 
              }}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic
              title="运行中"
              value={executions.filter(e => e.status === 'running').length}
              valueStyle={{ color: '#1890ff' }}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic
              title="平均时长"
              value={calculateAverageDuration(executions)}
              suffix="s"
              valueStyle={{ color: '#722ed1' }}
            />
          </Card>
        </Col>
      </Row>

      <Card>
        <div style={{ marginBottom: 16 }}>
          <Space wrap>
            <Select
              placeholder="选择项目"
              style={{ width: 200 }}
              allowClear
              value={filters.project_id}
              onChange={(value) => setFilters({ ...filters, project_id: value })}
            >
              {projects.map(project => (
                <Option key={project.id} value={project.id}>
                  {project.name}
                </Option>
              ))}
            </Select>
            
            <Select
              placeholder="选择环境"
              style={{ width: 150 }}
              allowClear
              value={filters.environment_id}
              onChange={(value) => setFilters({ ...filters, environment_id: value })}
            >
              {environments.map(env => (
                <Option key={env.id} value={env.id}>
                  {env.name}
                </Option>
              ))}
            </Select>
            
            <Select
              placeholder="执行类型"
              style={{ width: 120 }}
              allowClear
              value={filters.execution_type}
              onChange={(value) => setFilters({ ...filters, execution_type: value })}
            >
              <Option value="test_case">测试用例</Option>
              <Option value="test_suite">测试套件</Option>
            </Select>
            
            <Select
              placeholder="选择状态"
              style={{ width: 120 }}
              allowClear
              value={filters.status}
              onChange={(value) => setFilters({ ...filters, status: value })}
            >
              <Option value="success">成功</Option>
              <Option value="failed">失败</Option>
              <Option value="running">运行中</Option>
              <Option value="pending">等待中</Option>
              <Option value="cancelled">已取消</Option>
            </Select>
            
            <RangePicker
              value={filters.date_range}
              onChange={(dates) => setFilters({ ...filters, date_range: dates })}
              placeholder={['开始日期', '结束日期']}
            />
            
            <Button icon={<ReloadOutlined />} onClick={loadExecutions}>
              刷新
            </Button>
          </Space>
        </div>

        <Table
          columns={columns}
          dataSource={executions}
          rowKey="id"
          loading={loading}
          pagination={{
            ...pagination,
            showSizeChanger: true,
            showQuickJumper: true,
            showTotal: (total, range) =>
              `第 ${range[0]}-${range[1]} 条，共 ${total} 条`,
            onChange: (page, pageSize) => {
              setPagination({ ...pagination, current: page, pageSize: pageSize || 10 });
            },
          }}
          scroll={{ x: 1300 }}
        />
      </Card>
    </div>
  );
};

export default Executions;